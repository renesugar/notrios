//go:build windows

package synccarrier

import (
	"os"
	"syscall"
)

func singleLink(file *os.File, _ os.FileInfo) bool {
	var information syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &information); err != nil {
		return false
	}
	return information.NumberOfLinks == 1
}
