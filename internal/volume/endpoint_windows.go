//go:build windows

package volume

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const clsctxAll = windows.CLSCTX_INPROC_SERVER | windows.CLSCTX_INPROC_HANDLER | windows.CLSCTX_LOCAL_SERVER | windows.CLSCTX_REMOTE_SERVER

var (
	clsidMMDeviceEnumerator = windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidIAudioEndpointVolume = windows.GUID{Data1: 0x5CDF2C82, Data2: 0x841E, Data3: 0x4546, Data4: [8]byte{0x97, 0x22, 0x0C, 0xF7, 0x40, 0x78, 0x22, 0x9A}}
	procCoCreateInstance    = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")
)

type iUnknownVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

type mmDeviceEnumerator struct {
	vtbl *mmDeviceEnumeratorVtbl
}

type mmDeviceEnumeratorVtbl struct {
	iUnknownVtbl
	EnumAudioEndpoints             uintptr
	GetDefaultAudioEndpoint        uintptr
	GetDevice                      uintptr
	RegisterEndpointNotification   uintptr
	UnregisterEndpointNotification uintptr
}

type mmDevice struct {
	vtbl *mmDeviceVtbl
}

type mmDeviceVtbl struct {
	iUnknownVtbl
	Activate          uintptr
	OpenPropertyStore uintptr
	GetID             uintptr
	GetState          uintptr
}

type audioEndpointVolume struct {
	vtbl *audioEndpointVolumeVtbl
}

type audioEndpointVolumeVtbl struct {
	iUnknownVtbl
	RegisterControlChangeNotify   uintptr
	UnregisterControlChangeNotify uintptr
	GetChannelCount               uintptr
	SetMasterVolumeLevel          uintptr
	SetMasterVolumeLevelScalar    uintptr
	GetMasterVolumeLevel          uintptr
	GetMasterVolumeLevelScalar    uintptr
	SetChannelVolumeLevel         uintptr
	SetChannelVolumeLevelScalar   uintptr
	GetChannelVolumeLevel         uintptr
	GetChannelVolumeLevelScalar   uintptr
	SetMute                       uintptr
	GetMute                       uintptr
	GetVolumeStepInfo             uintptr
	VolumeStepUp                  uintptr
	VolumeStepDown                uintptr
	QueryHardwareSupport          uintptr
	GetVolumeRange                uintptr
}

func setSystemVolume(level float64) error {
	endpoint, cleanup, err := openDefaultEndpointVolume()
	if err != nil {
		return err
	}
	defer cleanup()

	level = math.Max(0, math.Min(1, level))
	r, _, _ := syscall.SyscallN(
		endpoint.vtbl.SetMasterVolumeLevelScalar,
		uintptr(unsafe.Pointer(endpoint)),
		uintptr(math.Float32bits(float32(level))),
		0,
	)
	if r != 0 {
		return hresultError("IAudioEndpointVolume.SetMasterVolumeLevelScalar", r)
	}
	return nil
}

func getSystemVolume() (float64, error) {
	endpoint, cleanup, err := openDefaultEndpointVolume()
	if err != nil {
		return 0, err
	}
	defer cleanup()

	var level float32
	r, _, _ := syscall.SyscallN(
		endpoint.vtbl.GetMasterVolumeLevelScalar,
		uintptr(unsafe.Pointer(endpoint)),
		uintptr(unsafe.Pointer(&level)),
	)
	if r != 0 {
		return 0, hresultError("IAudioEndpointVolume.GetMasterVolumeLevelScalar", r)
	}
	return math.Max(0, math.Min(1, float64(level))), nil
}

func openDefaultEndpointVolume() (*audioEndpointVolume, func(), error) {
	initialized := false
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err == nil || isHResult(err, uintptr(windows.S_FALSE)) {
		initialized = true
	} else if err != nil {
		return nil, nil, fmt.Errorf("volume: CoInitializeEx: %w", err)
	}

	cleanup := func() {
		if initialized {
			windows.CoUninitialize()
		}
	}

	var enumeratorPtr unsafe.Pointer
	r, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)),
		0,
		uintptr(clsctxAll),
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)),
		uintptr(unsafe.Pointer(&enumeratorPtr)),
	)
	if r != 0 {
		cleanup()
		return nil, nil, hresultError("CoCreateInstance(MMDeviceEnumerator)", r)
	}
	enumerator := (*mmDeviceEnumerator)(enumeratorPtr)

	var device *mmDevice
	r, _, _ = syscall.SyscallN(
		enumerator.vtbl.GetDefaultAudioEndpoint,
		uintptr(unsafe.Pointer(enumerator)),
		0,
		1,
		uintptr(unsafe.Pointer(&device)),
	)
	release(enumerator.vtbl.Release, unsafe.Pointer(enumerator))
	if r != 0 {
		cleanup()
		return nil, nil, hresultError("IMMDeviceEnumerator.GetDefaultAudioEndpoint", r)
	}

	var endpointPtr unsafe.Pointer
	r, _, _ = syscall.SyscallN(
		device.vtbl.Activate,
		uintptr(unsafe.Pointer(device)),
		uintptr(unsafe.Pointer(&iidIAudioEndpointVolume)),
		uintptr(clsctxAll),
		0,
		uintptr(unsafe.Pointer(&endpointPtr)),
	)
	release(device.vtbl.Release, unsafe.Pointer(device))
	if r != 0 {
		cleanup()
		return nil, nil, hresultError("IMMDevice.Activate(IAudioEndpointVolume)", r)
	}

	endpoint := (*audioEndpointVolume)(endpointPtr)
	return endpoint, func() {
		release(endpoint.vtbl.Release, unsafe.Pointer(endpoint))
		cleanup()
	}, nil
}

func release(fn uintptr, ptr unsafe.Pointer) {
	_, _, _ = syscall.SyscallN(fn, uintptr(ptr))
}

func hresultError(operation string, hresult uintptr) error {
	return fmt.Errorf("volume: %s failed with HRESULT 0x%08X", operation, uint32(hresult))
}

func isHResult(err error, hresult uintptr) bool {
	if errno, ok := err.(syscall.Errno); ok {
		return uintptr(errno) == hresult
	}
	return false
}
