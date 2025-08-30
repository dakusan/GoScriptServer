// Package settings handles loading and saving settings to the settings.json and getting/setting settings via Get() and Set()
package settings

import (
	"encoding/json"
	"os"
	"script_server/utils"

	"github.com/pkg/errors"
)

const FileName = "settings.json"

var vars map[string]map[string]string

func InitSettings() error {
	if vars != nil {
		return errors.New("Settings already initialized")
	}
	if data, err := os.ReadFile(FileName); err != nil {
		return err
	} else if err = json.Unmarshal(data, &vars); err != nil {
		return err
	}
	return saveSettings()
}

func saveSettings() error {
	if data, err := json.MarshalIndent(vars, "", "  "); err != nil {
		return errors.Errorf("Error saving json [convert]: %v", err)
	} else if err = os.WriteFile(FileName, data, 0644); err != nil {
		return errors.Errorf("Error saving json [write]: %v", err)
	}
	return nil
}

func Set(sectionName, varName, varValue string) {
	getSection, ok := vars[sectionName]
	if !ok {
		getSection = make(map[string]string)
		vars[sectionName] = getSection
	}

	getSection[varName] = varValue
	_ = saveSettings()
}

func Get(sectionName, varName, defaultVal string) string {
	if getSection, ok := vars[sectionName]; !ok {
	} else if ret, ok := getSection[varName]; ok {
		return ret
	}

	utils.PrintError("Setting %s.%s not found, using default: %s", sectionName, varName, defaultVal)
	return defaultVal
}
