package config

import "testing"

func TestRead(t *testing.T) {
	configInfo, err := Read()
	if err != nil {
		t.Errorf("Issue reading file: %v", err)
	}
	if len(configInfo.CurrentUserName) == 0 {
		t.Error("No username in struct returned by read function")
	}
}

func TestSetUser(t *testing.T) {
	var testConfig Config
	if err := testConfig.SetUser("bingo"); err != nil {
		t.Errorf("Issue setting user: %v", err)
	}
	afterStruct, err := Read()
	if err != nil {
		t.Errorf("Issue reading file after SetUser: %v", err)
	}
	if afterStruct.CurrentUserName != "bingo" {
		t.Errorf("Username is not 'bingo', instead got: %v", afterStruct.CurrentUserName)
	}
}
