//This is an example that runs a "beep" command
//Executes the command `settings.Beep.ScriptLocation` via `os.exec.Command()`

package plugins

import (
	"script_server/commands"
	"script_server/settings"
	"script_server/utils"
)

func init() {
	commands.Add("Beep", beepFunc)
}

func beepFunc(_ commands.GetQueryValFunc) string {
	ret, _ := utils.ExecCommand("Beep", settings.Get("Beep", "ScriptLocation", "/bin/beep"))
	return ret
}
