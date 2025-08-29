package plugin_volume

import (
	"fmt"
	"github.com/pkg/errors"
	"log"
	"regexp"
	"script_server/commands"
	"script_server/utils"
	"strconv"
	"strings"
)

type volumePlugin struct {
	/*This tracks buffered relative volume changes.
	Set to plus/minus direction * $BufferSize when $NormalVolumeMax is reached.

	Then decrements by 1 toward 0 per change. Volume increases past $NormalVolumeMax when 0 is reached.*/
	normalBuffer int

	//Cannot initialize constants and variables until after settings has loaded.
	//So this is not done until exec is called for the first time.
	hasInitialized bool

	currentVolume   int            //The current volume
	newVolumeRegEx  *regexp.Regexp //Used to parse URL parameter "NewVolume"
	runIndividually chan struct{}  //A mutex so only 1 of these commands runs at a time
}

var globalVP = volumePlugin{
	normalBuffer:    0,
	hasInitialized:  false,
	newVolumeRegEx:  utils.IgnoreError(regexp.Compile(`^([+-]?)(\d{1,3})$`)),
	runIndividually: make(chan struct{}, 1),
}

func init() {
	commands.Add("Volume", globalVP.funcWrapper)
}

func (vp *volumePlugin) funcWrapper(getQueryVal commands.GetQueryValFunc) string {
	//Only allow 1 to run at a time
	vp.runIndividually <- struct{}{}
	defer func() { <-vp.runIndividually }()
	return vp.exec(getQueryVal)
}

func (vp *volumePlugin) exec(getQueryVal commands.GetQueryValFunc) string {
	if !vp.hasInitialized {
		vp.initRunTime()
	}

	//Get the requested new volume/relative change
	if newVolStr, ok := getQueryVal("NewVolume"); !ok {
		return "Missing NewVolume"
	} else if match := vp.newVolumeRegEx.FindStringSubmatch(newVolStr); len(match) == 0 {
		return strings.ReplaceAll(`
Invalid NewVolume format. Pass a 1-3 digit integer, optionally preceded by a '+' or '-' sign. A '+' or '-'
 indicates a relative volume change from the current level, while no sign sets an absolute volume.`, "\n", "")
	} else if match[1] != "" { //Relative change
		vp.currentVolume = vp.calcNewVolume(
			utils.Cond(match[1] == "+", 1, -1),
			utils.IgnoreError(strconv.Atoi(match[2])),
		)
	} else if newVol, err := vp.verifyVolumeString(match[2]); err != nil {
		return fmt.Sprintf("Invalid absolute NewVolume: %s", err.Error())
	} else { //Absolute change
		vp.currentVolume = newVol
		vp.normalBuffer = 0
	}

	//Run the updates and return message
	if err := vp.updateSystemVolume(); err != nil {
		return fmt.Sprintf("Error settings new volume (NewVolume=%d, normalBuffer=%d): %s", vp.currentVolume, vp.normalBuffer, err.Error())
	}
	globalVb.Update()
	return fmt.Sprintf("NewVolume=%d, normalBuffer=%d", vp.currentVolume, vp.normalBuffer)
}

// Calculates the new volume from a relative change
func (vp *volumePlugin) calcNewVolume(direction, stepSize int) int {
	//0 delta does nothing
	curVolume := vp.currentVolume
	delta := direction * stepSize
	if delta == 0 {
		return curVolume
	}

	//Clear the normalBuffer and remember it for this function
	remNormalBuffer := vp.normalBuffer
	vp.normalBuffer = 0

	//Make sure the direction does not go below 0 or above OverMaxVolumeMax
	if curVolume+delta < 0 {
		return 0
	} else if curVolume+delta > vs.OverMaxVolumeMax {
		return vs.OverMaxVolumeMax
	}

	//When reaching NormalVolumeMax from either direction
	//add a buffer so it doesn't skip over NormalVolumeMax for BufferSize of the same direction
	newVolume := curVolume + delta
	switch {
	case
		curVolume < vs.NormalVolumeMax && newVolume >= vs.NormalVolumeMax,
		curVolume > vs.NormalVolumeMax && newVolume <= vs.NormalVolumeMax:
		remNormalBuffer = (vs.BufferSize + 1) * direction
	}

	//Return new volume if...
	switch {
	case
		remNormalBuffer == 0,                     //There is no normalBuffer
		(direction > 0) != (remNormalBuffer > 0): //Going a different direction than when the normalBuffer was created
		return newVolume
	}

	//Decrement the buffer (towards 0) until it reaches 0
	vp.normalBuffer = remNormalBuffer - direction
	//TODO: We should probably clear this buffer after an interval, but I'm not worrying about it

	//If normalBuffer has reached 0, continue the volume slide
	if vp.normalBuffer == 0 {
		return newVolume
	} else {
		return vs.NormalVolumeMax
	}
}

// Get the volume from the system
func (vp *volumePlugin) fetchSystemVolume() (int, error) {
	if curVolStr, err := utils.ExecCommandRaw(
		"bash", "-c",
		vs.GetCurVolumeCommand,
	); err != nil {
		return 0, err
	} else {
		return vp.verifyVolumeString(curVolStr)
	}
}

// Set the new volume to the system
func (vp *volumePlugin) updateSystemVolume() error {
	output, ok := utils.ExecCommand(
		"SetVolume", "bash", "-c",
		strings.ReplaceAll(vs.SetCurVolumeCommand, "$1", strconv.Itoa(vp.currentVolume)),
	)
	return utils.Cond(ok, nil, errors.New(output))
}

// Verify that an absolute volume value is valid
func (vp *volumePlugin) verifyVolumeString(volStr string) (int, error) {
	if !utils.IgnoreError(regexp.MatchString(`^\d{1,3}$`, volStr)) {
		return 0, errors.Errorf("Volume string is not a 1-3 digit integer: %s", volStr)
	} else if volInt, err := strconv.Atoi(volStr); err != nil {
		return 0, errors.Errorf("Volume string [%s] convert failed: %s", volStr, err.Error())
	} else if volInt > vs.OverMaxVolumeMax {
		return 0, errors.Errorf("Volume [%d] cannot be larger than %d", volInt, vs.OverMaxVolumeMax)
	} else {
		return volInt, nil
	}
}

func (vp *volumePlugin) initRunTime() {
	vp.hasInitialized = true
	loadSettings()

	//Get the current volume
	if curVol, err := vp.fetchSystemVolume(); err != nil {
		utils.PrintError("Error pulling current volume, setting to %d: %s", vs.DefaultVolume, err.Error())
		vp.currentVolume = vs.DefaultVolume
	} else {
		vp.currentVolume = curVol
	}
	log.Printf("Setting default volume at: %d\n", vp.currentVolume)

	//Position the volume window
	globalVb.PushCommand(vbCommandInitWindowAfterSettings)
}

func (vp *volumePlugin) GetCurrentVolume() int {
	return vp.currentVolume
}
