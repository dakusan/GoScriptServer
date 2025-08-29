package plugin_volume

import (
	"fmt"
	"image/color"
	"reflect"
	"regexp"
	"script_server/settings"
	"script_server/utils"
	"strconv"
	"strings"
)

var vs volumeSettings

// Volume settings loaded from the settings file
type volumeSettings = struct {
	//Volume threshold for relative changes.
	//Volume pauses at this value until $BufferSize relative change messages are received, then increases beyond.
	//Everything above this value is considered over-max.
	NormalVolumeMax int

	//Screen width (in pixels) used to center the volume bar.
	//Left position calculated as $VolumeBarLeftOffset+($ScreenWidth-$PercentPixelWidth*$OverMaxVolumeMax)/2
	ScreenWidth int

	//Run extra X windows functions that use "dangerous" behavior. If the programming is crashing, this is probably why.
	//This includes for the volume bar: Keep on top, pass mouse through, do not show on taskbar, and make non-focusable.
	RunExtraXWinCode int

	OverMaxVolumeMax    int        //Maximum volume in over-max mode (when above $NormalVolumeMax).
	DefaultVolume       int        //Fallback volume used if the system volume query fails.
	BufferSize          int        //The number of relative volume changes to buffer before passing $NormalVolumeMax.
	VolumeBarTop        int        //Y-axis position (in pixels) for the volume bar on the screen.
	VolumeBarLeftOffset int        //X-axis offset (in pixels) added to the volume bar's position.
	PercentPixelWidth   int        //Pixel width per percentage point of volume for the volume bar.
	VolumeBarHeight     int        //Volume bar height (in pixels)
	VolumeBarTimeout    int        //Number of idle milliseconds before hiding the volume bar
	TextSize            int        //The text size (and height)
	GetCurVolumeCommand string     //The bash command to get the current volume
	SetCurVolumeCommand string     //The bash command to set the current volume. Replaces $1 with the new volume
	FontPath            string     //Volume bar font path
	BGColor             color.RGBA //Color of the volume bar background
	VolColor            color.RGBA //Color of the volume bar current volume
	OverMaxColor        color.RGBA //Color of the volume bar over-max volume
	TextColor           color.RGBA //Color of the volume bar text
}

// Outputs additional debugging info
const isDebugging = false

func loadSettings() {
	//Declare the settings that need to be loaded
	type settingToLoad struct {
		settingName      string
		theDefault       any
		align            string
		normalizeValFunc func(varVal int) int
	}
	settingsToLoad := [...]settingToLoad{
		{"NormalVolumeMax    ", 100, " ", func(v int) int { return max(v, 1) }},
		{"BufferSize         ", 5, "   ", func(v int) int { return max(v, 0) }},
		{"OverMaxVolumeMax   ", 200, " ", func(v int) int { return max(v, vs.NormalVolumeMax) }},
		{"DefaultVolume      ", 50, "  ", func(v int) int { return max(0, min(v, vs.OverMaxVolumeMax)) }},
		{"VolumeBarHeight    ", 80, "  ", func(v int) int { return max(v, 1) }},
		{"VolumeBarTop       ", 30, "  ", nil},
		{"VolumeBarTimeout   ", 2000, "", func(v int) int { return max(v, 1) }},
		{"VolumeBarLeftOffset", 0, "   ", nil},
		{"PercentPixelWidth  ", 6, "   ", func(v int) int { return max(v, 1) }},
		{"ScreenWidth        ", 1920, "", func(v int) int { return max(v, vs.OverMaxVolumeMax*vs.PercentPixelWidth) }},
		{"TextSize           ", 64, "  ", func(v int) int { return max(v, 1) }},
		{"RunExtraXWinCode   ", 1, "   ", func(v int) int { return min(v, 1) }},
		{"BGColor            ", 0x00000034, "", nil},
		{"VolColor           ", 0x0000FF34, "", nil},
		{"OverMaxColor       ", 0xFF000034, "", nil},
		{"TextColor          ", 0xFFFFFFFF, "", nil},
		{"GetCurVolumeCommand", "pactl get-sink-volume @DEFAULT_SINK@ | grep -oP '[0-9]+(?=%)' | head -1", "", nil},
		{"SetCurVolumeCommand", "pactl set-sink-volume '@DEFAULT_SINK@' $1%", "", nil},
		{"FontPath            ", "/usr/share/fonts/truetype/freefont/FreeSans.ttf", "", nil},
	}

	//Load the settings
	settingsRef := reflect.ValueOf(&vs)
	for _, stl := range settingsToLoad {
		settingName := strings.TrimRight(stl.settingName, " ")

		//Handle different type values of settings
		fieldRef := settingsRef.Elem().FieldByName(settingName)
		settingVal := settings.Get("Volume", settingName, "")
		switch fieldRef.Kind() {
		case reflect.Int:
			var val int
			if newVal, err := strconv.Atoi(settingVal); err == nil {
				val = newVal
			} else {
				val = stl.theDefault.(int)
				utils.PrintError("Volume setting %s is not a valid int (%s) using default (%d)", settingName, settingVal, val)
			}
			if stl.normalizeValFunc != nil {
				val = stl.normalizeValFunc(val)
			}
			fieldRef.SetInt(int64(val))
		case reflect.String:
			if settingVal == "" {
				settingVal = stl.theDefault.(string)
				utils.PrintError("Volume setting %s is empty using default (%s)", settingName, settingVal)
			}
			fieldRef.SetString(settingVal)
		case reflect.Struct:
			if fieldRef.Type() != reflect.TypeOf(vs.BGColor) {
				panic("Invalid struct type")
			}
			if !utils.IgnoreError(regexp.MatchString(`^[0-9a-fA-F]{8}$`, settingVal)) {
				newSettingVal := fmt.Sprintf("%08x", stl.theDefault.(int))
				utils.PrintError("Volume setting %s is not a valid hex color (%s) using default (%s)", settingName, settingVal, newSettingVal)
				settingVal = newSettingVal
			}
			myIntVal := utils.IgnoreError(strconv.ParseUint(settingVal, 16, 32))
			fieldRef.Set(reflect.ValueOf(color.RGBA{
				R: uint8((myIntVal >> 24) & 0xFF),
				G: uint8((myIntVal >> 16) & 0xFF),
				B: uint8((myIntVal >> 8) & 0xFF),
				A: uint8(myIntVal & 0xFF),
			}))
		default:
			panic("unhandled default case")
		}
		if isDebugging {
			fmt.Printf("VOLUME SETTING: %s: %v\n", stl.settingName, fieldRef.Interface())
		}
	}
}
