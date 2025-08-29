//Opens a multi-file-selector dialog and executes a command with the selected files as parameters
//By default it opens or adds files to the celluloid music player playlist

package plugins

import (
	"fmt"
	"os"
	"script_server/commands"
	"script_server/settings"
	"script_server/utils"
	"strconv"
	"strings"
	"time"
)

const defaultFileFilters = "Playlists | *.m3u *.m3u8 & Music files | *.mp3 *.wav *.midi *.flac *.wma *.ogg & All files | *"
const invalidWindowPosDefault = 600

func init() {
	commands.Add("OpenFiles", openFilesFunc)
}

func openFilesFunc(getQueryVal commands.GetQueryValFunc) string {
	//Determine if opening or adding files
	var typeIsOpen bool
	if openTypeStr, ok := getQueryVal("OpenType"); !ok {
		return "Missing OpenType"
	} else if openTypeStr == "Open" {
		typeIsOpen = true
	} else if openTypeStr == "Add" {
		typeIsOpen = false
	} else {
		return "Invalid OpenType (Must be 'Add' or 'Open')"
	}

	//Resize the dialog
	dialogName := settingOF("DialogName", "Music")
	windowName := fmt.Sprintf(
		"%s %s",
		dialogName,
		utils.Cond(typeIsOpen, "Open", "Add"),
	)
	go func() {
		//Wait for the dialog to show up
		for {
			if _, err := utils.ExecCommandRaw("xdotool", "search", "--name", windowName); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		//Get the dialog geometry values. For any invalid value, invalidWindowPosDefault is used
		windowParams := []any{1}
		for _, settingName := range [...]string{"DialogLeft", "DialogTop", "DialogWidth", "DialogHeight"} {
			newVal, err := strconv.Atoi(settingOF(settingName, "!"))
			windowParams = append(windowParams, utils.Cond(err == nil, newVal, invalidWindowPosDefault))
		}

		//Run the resize
		utils.ExecCommand(
			"Move window", "wmctrl",
			"-r", windowName,
			"-e", fmt.Sprintf("%d,%d,%d,%d,%d", windowParams...),
		)
	}()

	//Get the path (if none exists in the settings, set to the current working directory)
	filePath := settingOF("OpenPath", "!DEFAULT!")
	if filePath == "!DEFAULT!" {
		if filePathTmp, err := os.Getwd(); err == nil {
			filePath = filePathTmp
		} else {
			filePath = ""
		}
	}
	filePath = strings.TrimRight(filePath, "/") + "/"

	//Get the file filters
	zenityParameters := []string{
		"--file-selection", "--multiple",
		"--filename=" + filePath,
		"--title=" + windowName,
	}
	for _, s := range strings.Split(settingOF("FileFilters", defaultFileFilters), "&") {
		zenityParameters = append(zenityParameters, "--file-filter="+strings.Trim(s, " "))
	}

	//Get the file list from a dialog
	var fileList []string
	if fileListStr, err := utils.ExecCommandRaw("zenity", zenityParameters[:]...); err != nil {
		return fmt.Sprintf("File selection cancelled (%v): %s", err, fileListStr)
	} else if fileListStr == "" {
		return "No items in list"
	} else {
		fileList = strings.Split(fileListStr, "|")
	}

	//Get the path (all files should have the same base path)
	var basePath string
	if lastSlash := strings.LastIndex(fileList[0], "/"); lastSlash == -1 {
		return "Path has no '/'"
	} else {
		basePath = fileList[0][:lastSlash+1]
	}
	settings.Set("OpenFiles", "OpenPath", basePath)

	//Validate and extract filenames
	var fileNames []string
	for _, f := range fileList {
		if !strings.HasPrefix(f, basePath) {
			return fmt.Sprintf("Path mismatch: %s != %s", f, basePath)
		} else if rest := f[len(basePath):]; strings.Contains(rest, "/") {
			return fmt.Sprintf("More than one slash found in filename: %s, [base=%s]", f, basePath)
		} else if rest != "" {
			fileNames = append(fileNames, rest)
		}
	}
	outputFileList := fmt.Sprintf("%s Files [%s]: %s", dialogName, basePath, strings.Join(fileNames, ", "))

	//Compile the new base path
	cmdBasePath := settingOF("PathPrepend", "")
	if directorySeparator := settingOF("DirectorySeparator", "/"); directorySeparator == "/" {
		cmdBasePath += basePath
	} else {
		cmdBasePath += strings.ReplaceAll(basePath, "/", "\\")
	}

	//Create the command parameters
	cmdParams := make([]string, 0, len(fileList)+1)
	if !typeIsOpen {
		cmdParams = append(cmdParams, settingOF("AppendCommand", "--enqueue"))
	}
	for _, f := range fileNames { //Add paths as command parameters
		cmdParams = append(cmdParams, cmdBasePath+f)
	}

	//Execute the command
	if output, ok := utils.ExecCommand("ExecCommand", settingOF("ExecCommand", "/usr/bin/celluloid"), cmdParams[:]...); !ok {
		return fmt.Sprintf("%s :: %s", output, outputFileList)
	}

	return outputFileList
}

func settingOF(varName, defaultVal string) string {
	return settings.Get("OpenFiles", varName, defaultVal)
}
