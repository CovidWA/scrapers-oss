package csg

//happy path testing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAkamaiSensorData(t *testing.T) {
	var sensorDataFilePaths []string

	root := "./"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".yaml" && strings.HasSuffix(path, "sensor_data.yaml") {
			sensorDataFilePaths = append(sensorDataFilePaths, path)
		}
		return nil
	})
	if err != nil {
		t.Errorf("Unexpected Error: %v", err)
		return
	}

	for _, path := range sensorDataFilePaths {
		Log.Infof("Testing: %s", path)

		asd := ParseAkamaiSensorData(path)

		for range asd.ItemList {
			sdi, err := asd.GetSensorData()
			if err != nil {
				t.Errorf("Unexpected Error: %v", err)
				return
			}

			sdi.MarkUsed()
		}

		_, err = asd.GetSensorData()
		if err == nil {
			t.Errorf("Expecting error but got nil")
			return
		}
	}
}
