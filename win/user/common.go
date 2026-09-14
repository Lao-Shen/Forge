package user

import "os"

// UserHomeDir Dir:C:\Users\Administrator
func UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

// AppDataRoaming Dir:C:\Users\Administrator\AppData\Roaming
func AppDataRoaming() string {
	return os.Getenv("APPDATA")
}

// AppDataLocal Dir:C:\Users\Administrator\AppData\Local
func AppDataLocal() string {
	return os.Getenv("LOCALAPPDATA")
}
