package winsys

import (
	"errors"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Service struct {
	Name    string
	Display string
}

func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func Processes() (map[string]int, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	out := map[string]int{}
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		out[strings.ToLower(windows.UTF16ToString(e.ExeFile[:]))]++
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return out, nil
}

func ActiveServices() ([]Service, error) {
	h, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return nil, err
	}
	defer windows.CloseServiceHandle(h)

	size := uint32(64 * 1024)
	for {
		buf := make([]byte, size)
		var needed, count, resume uint32
		err := windows.EnumServicesStatusEx(h, windows.SC_ENUM_PROCESS_INFO, windows.SERVICE_WIN32,
			windows.SERVICE_ACTIVE, &buf[0], size, &needed, &count, &resume, nil)
		if errors.Is(err, windows.ERROR_MORE_DATA) {
			size += needed
			continue
		}
		if err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, nil
		}
		entries := unsafe.Slice((*windows.ENUM_SERVICE_STATUS_PROCESS)(unsafe.Pointer(&buf[0])), count)
		out := make([]Service, 0, count)
		for _, e := range entries {
			out = append(out, Service{
				Name:    windows.UTF16PtrToString(e.ServiceName),
				Display: windows.UTF16PtrToString(e.DisplayName),
			})
		}
		return out, nil
	}
}
