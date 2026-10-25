//go:build windows

// Portable tray translator. Build with: go build -trimpath -ldflags="-s -w -H=windowsgui" -o Honyaku-v1.0.1.exe .
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const appVersion = "1.0.1"

const (
	wmApp           = 0x8000
	wmTray          = wmApp + 1
	wmTranslateDone = wmApp + 2
	wmSubmit        = wmApp + 3
	wmReload        = wmApp + 4
	wmHotkey        = 0x0312
	wmCommand       = 0x0111
	wmDestroy       = 0x0002
	wmKeyDown       = 0x0100
	wmClose         = 0x0010
	wmTimer         = 0x0113
	wmSetFocus      = 0x0007
	wmUser          = 0x0400
	wsPopup         = 0x80000000
	wsVisible       = 0x10000000
	wsChild         = 0x40000000
	wsTabStop       = 0x00010000
	wsBorder        = 0x00800000
	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	editAutoHScroll = 0x0080
	editWantReturn  = 0x1000
	modAlt          = 0x0001
	modControl      = 0x0002
	modShift        = 0x0004
	modWin          = 0x0008
	modNoRepeat     = 0x4000
	trayIconID      = 1
	menuSettings    = 1001
	menuReload      = 1002
	menuStartup     = 1003
	menuExit        = 1004
	cfUnicodeText   = 13
	gmemMoveable    = 0x0002
	swHide          = 0
	swShow          = 5
	swRestore       = 9
	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpShowWindow   = 0x0040
	swpNoActivate   = 0x0010
	topmostTimerID  = 2
	msgfClose       = 0x0010
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	shell32                    = syscall.NewLazyDLL("shell32.dll")
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	advapi32                   = syscall.NewLazyDLL("advapi32.dll")
	imm32                      = syscall.NewLazyDLL("imm32.dll")
	procRegisterClass          = user32.NewProc("RegisterClassW")
	procCreateWindow           = user32.NewProc("CreateWindowExW")
	procDefWindowProc          = user32.NewProc("DefWindowProcW")
	procGetMessage             = user32.NewProc("GetMessageW")
	procTranslateMsg           = user32.NewProc("TranslateMessage")
	procDispatchMsg            = user32.NewProc("DispatchMessageW")
	procPostMessage            = user32.NewProc("PostMessageW")
	procShowWindow             = user32.NewProc("ShowWindow")
	procUpdateWindow           = user32.NewProc("UpdateWindow")
	procSetForeground          = user32.NewProc("SetForegroundWindow")
	procSetFocus               = user32.NewProc("SetFocus")
	procSetWindowPos           = user32.NewProc("SetWindowPos")
	procSetTimer               = user32.NewProc("SetTimer")
	procKillTimer              = user32.NewProc("KillTimer")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procRegisterHotkey         = user32.NewProc("RegisterHotKey")
	procUnregisterHK           = user32.NewProc("UnregisterHotKey")
	procLoadCursor             = user32.NewProc("LoadCursorW")
	procLoadIcon               = user32.NewProc("LoadIconW")
	procGetModule              = kernel32.NewProc("GetModuleHandleW")
	procCreateMutex            = kernel32.NewProc("CreateMutexW")
	procGetCursorPos           = user32.NewProc("GetCursorPos")
	procGetSystemMetric        = user32.NewProc("GetSystemMetrics")
	procCreatePopup            = user32.NewProc("CreatePopupMenu")
	procAppendMenu             = user32.NewProc("AppendMenuW")
	procTrackPopup             = user32.NewProc("TrackPopupMenu")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procGetWindowText          = user32.NewProc("GetWindowTextW")
	procSetWindowText          = user32.NewProc("SetWindowTextW")
	procSetWindowLong          = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProc         = user32.NewProc("CallWindowProcW")
	procEnableWindow           = user32.NewProc("EnableWindow")
	procGetClientRect          = user32.NewProc("GetClientRect")
	procShellNotify            = shell32.NewProc("Shell_NotifyIconW")
	procOpenClipboard          = user32.NewProc("OpenClipboard")
	procEmptyClipboard         = user32.NewProc("EmptyClipboard")
	procSetClipboard           = user32.NewProc("SetClipboardData")
	procCloseClipboard         = user32.NewProc("CloseClipboard")
	procGlobalAlloc            = kernel32.NewProc("GlobalAlloc")
	procGlobalLock             = kernel32.NewProc("GlobalLock")
	procGlobalUnlock           = kernel32.NewProc("GlobalUnlock")
	procGlobalFree             = kernel32.NewProc("GlobalFree")
	procCopyMemory             = kernel32.NewProc("RtlMoveMemory")
	procPostQuit               = user32.NewProc("PostQuitMessage")
	procShellExecute           = shell32.NewProc("ShellExecuteW")
	procRegisterMessage        = user32.NewProc("RegisterWindowMessageW")
	procGetForeground          = user32.NewProc("GetForegroundWindow")
	procGetWindowThread        = user32.NewProc("GetWindowThreadProcessId")
	procGetCurrentThread       = kernel32.NewProc("GetCurrentThreadId")
	procGetKeyboardLayout      = user32.NewProc("GetKeyboardLayout")
	procLoadKeyboardLayout     = user32.NewProc("LoadKeyboardLayoutW")
	procActivateKeyboardLayout = user32.NewProc("ActivateKeyboardLayout")
	procAttachInput            = user32.NewProc("AttachThreadInput")
	procBringToTop             = user32.NewProc("BringWindowToTop")
	procSetActive              = user32.NewProc("SetActiveWindow")
	procImmGetContext          = imm32.NewProc("ImmGetContext")
	procImmSetOpen             = imm32.NewProc("ImmSetOpenStatus")
	procImmSetConversion       = imm32.NewProc("ImmSetConversionStatus")
	procImmRelease             = imm32.NewProc("ImmReleaseContext")
	procRegOpenKey             = advapi32.NewProc("RegOpenKeyExW")
	procRegCreateKey           = advapi32.NewProc("RegCreateKeyExW")
	procRegQueryValue          = advapi32.NewProc("RegQueryValueExW")
	procRegSetValue            = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValue         = advapi32.NewProc("RegDeleteValueW")
	procRegCloseKey            = advapi32.NewProc("RegCloseKey")
	windowProcCallback         uintptr
	editProcCallback           uintptr
	oldEditProc                uintptr
	taskbarCreated             uint32
	app                        = &application{}
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type msg struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
}
type wndClass struct {
	Style                              uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
}
type notifyIconData struct {
	Size             uint32
	Hwnd             uintptr
	ID               uint32
	Flags            uint32
	CallbackMessage  uint32
	Icon             uintptr
	Tip              [128]uint16
	State, StateMask uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	GUID             [16]byte
	BalloonIcon      uintptr
}
type config struct {
	API        string `json:"api"`
	APIKey     string `json:"api_key"`
	Hotkey     string `json:"hotkey"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}
type application struct {
	hwnd, popup, edit, label uintptr
	configPath               string
	cfg                      config
	iconData                 notifyIconData
	hotkeyMods               uint32
	hotkeyVK                 uint32
	busy                     bool
	resultCh                 chan translationResult
	mutex                    uintptr
	hasHotkey                bool
	requestCancel            context.CancelFunc
	requestID                uint64
	originalLayout           uintptr
	japaneseLayout           uintptr
}

func loword(v uintptr) uint16        { return uint16(v & 0xffff) }
func hiword(v uintptr) uint16        { return uint16((v >> 16) & 0xffff) }
func w(s string) *uint16             { p, _ := syscall.UTF16PtrFromString(s); return p }
func callOK(r uintptr, e error) bool { return r != 0 && e == nil }

func main() {
	runtime.LockOSThread()
	exe, err := os.Executable()
	if err != nil {
		return
	}
	mutexHandle, _, mutexErr := procCreateMutex.Call(0, 1, uintptr(unsafe.Pointer(w("Local\\HonyakuPortableTranslator.Singleton"))))
	app.mutex = mutexHandle
	if app.mutex == 0 {
		return
	}
	if mutexErr == syscall.Errno(183) {
		return
	}
	app.configPath = filepath.Join(filepath.Dir(exe), "config.json")
	if err := app.loadConfig(); err != nil {
		app.cfg = config{API: "deepl", Hotkey: "Ctrl+Shift+Q", SourceLang: "JA", TargetLang: "EN"}
		app.logError("read configuration", err)
	}
	instance, _, _ := procGetModule.Call(0)
	className := w("HonyakuPortableTranslator")
	windowProcCallback = syscall.NewCallback(windowProc)
	editProcCallback = syscall.NewCallback(editProc)
	taskbarMsg, _, _ := procRegisterMessage.Call(uintptr(unsafe.Pointer(w("TaskbarCreated"))))
	taskbarCreated = uint32(taskbarMsg)
	app.resultCh = make(chan translationResult, 1)
	app.originalLayout, _, _ = procGetKeyboardLayout.Call(0)
	cursor, _, _ := procLoadCursor.Call(0, 32512) // IDC_ARROW
	icon, _, _ := procLoadIcon.Call(0, 32512)     // IDI_APPLICATION
	wc := wndClass{WndProc: windowProcCallback, Instance: instance, Cursor: cursor, Icon: icon, ClassName: className}
	if r, _, e := procRegisterClass.Call(uintptr(unsafe.Pointer(&wc))); r == 0 && e != syscall.Errno(1410) {
		panic(fmt.Sprintf("RegisterClassW: %v", e))
	}
	app.hwnd, _, _ = procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(w(""))), 0, 0, 0, 0, 0, 0, 0, instance, 0)
	if app.hwnd == 0 {
		panic("could not create message window")
	}
	app.addTrayIcon(icon)
	if err := app.registerHotkey(); err != nil {
		app.logError("register hotkey", err)
	}
	var m msg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMsg.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsg.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (a *application) loadConfig() error {
	candidate := config{API: "deepl", Hotkey: "Ctrl+Shift+Q", SourceLang: "JA", TargetLang: "EN"}
	b, err := os.ReadFile(a.configPath)
	if os.IsNotExist(err) {
		a.cfg = candidate
		a.saveConfig()
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &candidate); err != nil {
		return fmt.Errorf("config.json の JSON が不正です: %w", err)
	}
	if strings.TrimSpace(candidate.API) == "" {
		candidate.API = "deepl"
	}
	if strings.TrimSpace(candidate.Hotkey) == "" {
		candidate.Hotkey = "Ctrl+Shift+Q"
	}
	if strings.TrimSpace(candidate.SourceLang) == "" {
		candidate.SourceLang = "JA"
	}
	if strings.TrimSpace(candidate.TargetLang) == "" {
		candidate.TargetLang = "EN"
	}
	if _, _, ok := parseHotkey(candidate.Hotkey); !ok {
		return fmt.Errorf("未対応のホットキーです: %s", candidate.Hotkey)
	}
	a.cfg = candidate
	return nil
}
func (a *application) saveConfig() {
	b, _ := json.MarshalIndent(a.cfg, "", "  ")
	_ = os.WriteFile(a.configPath, append(b, '\n'), 0600)
}
func (a *application) logError(where string, err error) {
	if err == nil || a.configPath == "" {
		return
	}
	line := fmt.Sprintf("%s %s: %v\r\n", time.Now().Format(time.RFC3339), where, err)
	f, openErr := os.OpenFile(filepath.Join(filepath.Dir(a.configPath), "Honyaku.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if openErr != nil {
		return
	}
	_, _ = f.WriteString(line)
	_ = f.Close()
}
func (a *application) registerHotkey() error {
	mods, vk, ok := parseHotkey(a.cfg.Hotkey)
	if !ok {
		return fmt.Errorf("未対応のホットキーです: %s", a.cfg.Hotkey)
	}
	oldMods, oldVK, hadOld := a.hotkeyMods, a.hotkeyVK, a.hasHotkey
	if hadOld {
		procUnregisterHK.Call(a.hwnd, 1)
	}
	r, _, e := procRegisterHotkey.Call(a.hwnd, 1, uintptr(mods|modNoRepeat), uintptr(vk))
	if r == 0 {
		if hadOld {
			if restored, _, _ := procRegisterHotkey.Call(a.hwnd, 1, uintptr(oldMods|modNoRepeat), uintptr(oldVK)); restored != 0 {
				a.hotkeyMods, a.hotkeyVK, a.hasHotkey = oldMods, oldVK, true
			} else {
				a.hasHotkey = false
			}
		}
		return fmt.Errorf("RegisterHotKey に失敗しました（別アプリとの重複など）: %v", e)
	}
	a.hotkeyMods, a.hotkeyVK, a.hasHotkey = mods, vk, true
	return nil
}
func parseHotkey(s string) (uint32, uint32, bool) {
	var mods uint32
	var vk uint32
	parts := strings.Split(strings.ToUpper(strings.ReplaceAll(s, " ", "")), "+")
	for _, p := range parts {
		switch p {
		case "CTRL", "CONTROL":
			mods |= modControl
		case "ALT":
			mods |= modAlt
		case "SHIFT":
			mods |= modShift
		case "WIN", "WINDOWS":
			mods |= modWin
		default:
			if len(p) == 1 && p[0] >= 'A' && p[0] <= 'Z' {
				vk = uint32(p[0])
			} else if len(p) == 1 && p[0] >= '0' && p[0] <= '9' {
				vk = uint32(p[0])
			} else if strings.HasPrefix(p, "F") {
				var n int
				_, err := fmt.Sscanf(p, "F%d", &n)
				if err == nil && n >= 1 && n <= 24 {
					vk = 0x70 + uint32(n-1)
				}
			}
		}
	}
	return mods, vk, vk != 0 && mods != 0
}

func (a *application) addTrayIcon(icon uintptr) {
	data := notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Hwnd: a.hwnd, ID: trayIconID, Flags: 1 | 2 | 4, CallbackMessage: wmTray, Icon: icon}
	copy(data.Tip[:], syscall.StringToUTF16("Honyaku Translator v"+appVersion))
	a.iconData = data
	if r, _, e := procShellNotify.Call(0, uintptr(unsafe.Pointer(&a.iconData))); r == 0 {
		a.logError("add tray icon", e)
	}
}
func (a *application) showMenu() {
	menu, _, _ := procCreatePopup.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	appendItem := func(flags uint32, id uintptr, label string) {
		procAppendMenu.Call(menu, uintptr(flags), id, uintptr(unsafe.Pointer(w(label))))
	}
	appendItem(0, menuSettings, "設定ファイルを開く")
	appendItem(0, menuReload, "設定を再読み込み")
	startup := "Windows起動時に実行"
	if a.startupEnabled() {
		startup = "✓ Windows起動時に実行"
	}
	appendItem(0, menuStartup, startup)
	appendItem(0x800, 0, "")
	appendItem(0, menuExit, "終了")
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	// Required for a tray context menu to dismiss when the user clicks elsewhere.
	procSetForeground.Call(a.hwnd)
	command, _, _ := procTrackPopup.Call(menu, 0x0102, uintptr(p.X), uintptr(p.Y), 0, a.hwnd, 0) // TPM_RETURNCMD | TPM_RIGHTBUTTON
	procPostMessage.Call(a.hwnd, 0, 0, 0)                                                        // WM_NULL: close the tray menu's modal loop cleanly
	if command != 0 {
		a.handleMenuCommand(uint16(command))
	}
}
func (a *application) handleMenuCommand(command uint16) {
	switch command {
	case menuSettings:
		a.openSettings()
	case menuReload:
		oldConfig := a.cfg
		if err := a.loadConfig(); err != nil {
			a.logError("reload configuration", err)
			return
		}
		if err := a.registerHotkey(); err != nil {
			a.cfg = oldConfig
			a.logError("register reloaded hotkey", err)
		}
	case menuStartup:
		a.toggleStartup()
	case menuExit:
		procDestroyWindow.Call(a.hwnd)
	}
}
func (a *application) openSettings() {
	if _, err := os.Stat(a.configPath); err != nil {
		a.saveConfig()
	}
	procShellExecute.Call(a.hwnd, uintptr(unsafe.Pointer(w("open"))), uintptr(unsafe.Pointer(w("notepad.exe"))), uintptr(unsafe.Pointer(w(a.configPath))), 0, swShow)
}
func (a *application) startupEnabled() bool {
	key, ok := openRunKey(1)
	if !ok {
		return false
	}
	defer procRegCloseKey.Call(key)
	var valueType uint32
	r, _, _ := procRegQueryValue.Call(key, uintptr(unsafe.Pointer(w("HonyakuPortable"))), 0, uintptr(unsafe.Pointer(&valueType)), 0, 0)
	return r == 0 && valueType == 1 // REG_SZ
}
func (a *application) toggleStartup() {
	if a.startupEnabled() {
		if key, ok := openRunKey(2); ok {
			procRegDeleteValue.Call(key, uintptr(unsafe.Pointer(w("HonyakuPortable"))))
			procRegCloseKey.Call(key)
		}
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	quoted := `"` + exe + `"`
	value, err := syscall.UTF16FromString(quoted)
	if err != nil {
		return
	}
	key, status := createRunKey()
	if status != 0 {
		a.logError("enable startup", syscall.Errno(status))
		return
	}
	defer procRegCloseKey.Call(key)
	status, _, _ = procRegSetValue.Call(key, uintptr(unsafe.Pointer(w("HonyakuPortable"))), 0, 1, uintptr(unsafe.Pointer(&value[0])), uintptr(len(value)*2))
	if status != 0 {
		a.logError("enable startup", syscall.Errno(status))
	}
}

const (
	hkeyCurrentUser = ^uintptr(0x7ffffffe)
	keyQueryValue   = 0x0001
	keySetValue     = 0x0002
	keyCreateSubKey = 0x0004
)

func openRunKey(access uintptr) (uintptr, bool) {
	var key uintptr
	r, _, _ := procRegOpenKey.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(w(`Software\Microsoft\Windows\CurrentVersion\Run`))), 0, access, uintptr(unsafe.Pointer(&key)))
	return key, r == 0
}
func createRunKey() (uintptr, uintptr) {
	var key, disposition uintptr
	r, _, _ := procRegCreateKey.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(w(`Software\Microsoft\Windows\CurrentVersion\Run`))), 0, 0, 0, keySetValue|keyCreateSubKey, 0, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&disposition)))
	return key, r
}

func (a *application) showPopup() {
	if a.popup != 0 {
		if a.busy {
			if a.requestCancel != nil {
				a.requestCancel()
				a.requestCancel = nil
			}
			a.requestID++
			a.busy = false
			procSetWindowText.Call(a.edit, uintptr(unsafe.Pointer(w(""))))
			a.setPopupHint("Enterで翻訳 / Escで閉じる")
		}
		procEnableWindow.Call(a.edit, 1)
		procShowWindow.Call(a.popup, swShow)
		procSetWindowPos.Call(a.popup, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
		procSetTimer.Call(a.hwnd, topmostTimerID, 250, 0)
		a.focusInput()
		return
	}
	instance, _, _ := procGetModule.Call(0)
	a.popup, _, _ = procCreateWindow.Call(wsExTopmost|wsExToolWindow, uintptr(unsafe.Pointer(w("STATIC"))), uintptr(unsafe.Pointer(w(""))), wsPopup|wsBorder, 100, 100, 440, 82, a.hwnd, 0, instance, 0)
	if a.popup == 0 {
		a.logError("create input popup", fmt.Errorf("CreateWindowExW failed"))
		return
	}
	a.label, _, _ = procCreateWindow.Call(0, uintptr(unsafe.Pointer(w("STATIC"))), uintptr(unsafe.Pointer(w("Enterで翻訳 / Escで閉じる"))), wsChild|wsVisible, 12, 7, 400, 18, a.popup, 2, instance, 0)
	a.edit, _, _ = procCreateWindow.Call(0, uintptr(unsafe.Pointer(w("EDIT"))), uintptr(unsafe.Pointer(w(""))), wsChild|wsVisible|wsTabStop|wsBorder|editAutoHScroll, 12, 30, 400, 30, a.popup, 1, instance, 0)
	oldEditProc, _, _ = procSetWindowLong.Call(a.edit, ^uintptr(3), editProcCallback) // GWLP_WNDPROC (-4)
	if a.edit == 0 || oldEditProc == 0 {
		a.logError("create input control", fmt.Errorf("EDIT control initialization failed"))
		procDestroyWindow.Call(a.popup)
		a.popup, a.edit, a.label = 0, 0, 0
		return
	}
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	sw, _, _ := procGetSystemMetric.Call(0)
	sh, _, _ := procGetSystemMetric.Call(1)
	x, y := int(p.X), int(p.Y)
	if x+440 > int(sw) {
		x = int(sw) - 440
	}
	if y+82 > int(sh) {
		y = int(sh) - 82
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	procSetWindowPos.Call(a.popup, ^uintptr(0), uintptr(x), uintptr(y), 440, 82, swpShowWindow)
	procSetTimer.Call(a.hwnd, topmostTimerID, 250, 0)
	procShowWindow.Call(a.popup, swShow)
	procUpdateWindow.Call(a.popup)
	a.focusInput()
}
func (a *application) setPopupHint(text string) {
	if a.label != 0 {
		procSetWindowText.Call(a.label, uintptr(unsafe.Pointer(w(text))))
	}
}
func (a *application) focusInput() {
	if a.japaneseLayout == 0 {
		a.japaneseLayout, _, _ = procLoadKeyboardLayout.Call(uintptr(unsafe.Pointer(w("00000411"))), 1) // Japanese IME, activate for this thread
	}
	if a.japaneseLayout != 0 {
		procActivateKeyboardLayout.Call(a.japaneseLayout, 0)
	} else {
		a.logError("activate Japanese IME", fmt.Errorf("Japanese keyboard layout is not available"))
	}
	foreground, _, _ := procGetForeground.Call()
	foregroundThread, _, _ := procGetWindowThread.Call(foreground, 0)
	currentThread, _, _ := procGetCurrentThread.Call()
	attached := false
	if foregroundThread != 0 && foregroundThread != currentThread {
		if r, _, _ := procAttachInput.Call(foregroundThread, currentThread, 1); r != 0 {
			attached = true
		}
	}
	procSetForeground.Call(a.popup)
	procBringToTop.Call(a.popup)
	procSetActive.Call(a.popup)
	procSetFocus.Call(a.edit)
	procSetWindowPos.Call(a.popup, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow|swpNoActivate)
	if attached {
		procAttachInput.Call(foregroundThread, currentThread, 0)
	}
	if imc, _, _ := procImmGetContext.Call(a.edit); imc != 0 {
		procImmSetOpen.Call(imc, 1)
		procImmSetConversion.Call(imc, 1, 0) // IME_CMODE_NATIVE: Japanese input
		procImmRelease.Call(a.edit, imc)
	}
}
func (a *application) hidePopup() {
	if a.requestCancel != nil {
		a.requestCancel()
		a.requestCancel = nil
	}
	a.requestID++
	if a.popup != 0 {
		procKillTimer.Call(a.hwnd, topmostTimerID)
		procShowWindow.Call(a.popup, swHide)
		procEnableWindow.Call(a.edit, 1)
		procSetWindowText.Call(a.edit, uintptr(unsafe.Pointer(w(""))))
	}
	if a.originalLayout != 0 {
		procActivateKeyboardLayout.Call(a.originalLayout, 0)
	}
	a.busy = false
}
func (a *application) submit() {
	if a.busy || a.edit == 0 {
		return
	}
	buf := make([]uint16, 8192)
	n, _, _ := procGetWindowText.Call(a.edit, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	text := syscall.UTF16ToString(buf[:n])
	if strings.TrimSpace(text) == "" {
		a.hidePopup()
		return
	}
	a.busy = true
	cfg := a.cfg
	a.requestID++
	requestID := a.requestID
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	a.requestCancel = cancel
	go func() {
		translated, err := translate(ctx, cfg, text)
		if ctx.Err() == context.Canceled {
			return
		}
		payload := translationResult{Text: translated, Err: "", RequestID: requestID}
		if err != nil {
			payload.Err = err.Error()
		}
		app.resultCh <- payload
		procPostMessage.Call(a.hwnd, wmTranslateDone, 0, 0)
	}()
}

type translationResult struct {
	Text      string `json:"text"`
	Err       string `json:"err"`
	RequestID uint64 `json:"request_id"`
}

func translate(ctx context.Context, c config, text string) (string, error) {
	if strings.ToLower(c.API) != "deepl" {
		return "", fmt.Errorf("未対応のAPIです。api は deepl を指定してください")
	}
	key := strings.TrimSpace(c.APIKey)
	if key == "" {
		return "", fmt.Errorf("config.json に DeepL API キーを設定してください")
	}
	endpoint := "https://api-free.deepl.com/v2/translate"
	if !strings.HasSuffix(key, ":fx") {
		endpoint = "https://api.deepl.com/v2/translate"
	}
	form := url.Values{"text": {text}, "source_lang": {strings.ToUpper(c.SourceLang)}, "target_lang": {strings.ToUpper(c.TargetLang)}}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "DeepL-Auth-Key "+key)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("DeepL API: %s", strings.TrimSpace(string(body)))
	}
	var out struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if len(out.Translations) == 0 {
		return "", fmt.Errorf("翻訳結果がありません")
	}
	return out.Translations[0].Text, nil
}
func copyClipboard(s string) error {
	data, err := syscall.UTF16FromString(s)
	if err != nil {
		return err
	}
	n := uintptr(len(data) * 2)
	h, _, _ := procGlobalAlloc.Call(gmemMoveable, n)
	if h == 0 {
		return fmt.Errorf("メモリを確保できません")
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return fmt.Errorf("メモリをロックできません")
	}
	procCopyMemory.Call(p, uintptr(unsafe.Pointer(&data[0])), n)
	procGlobalUnlock.Call(h)
	for i := 0; i < 20; i++ {
		r, _, _ := procOpenClipboard.Call(app.hwnd)
		if r != 0 {
			break
		}
		time.Sleep(15 * time.Millisecond)
		if i == 19 {
			procGlobalFree.Call(h)
			return fmt.Errorf("クリップボードを開けません")
		}
	}
	defer procCloseClipboard.Call()
	if r, _, _ := procEmptyClipboard.Call(); r == 0 {
		procCloseClipboard.Call()
		procGlobalFree.Call(h)
		return fmt.Errorf("クリップボードを空にできません")
	}
	if r, _, _ := procSetClipboard.Call(cfUnicodeText, h); r == 0 {
		procGlobalFree.Call(h)
		return fmt.Errorf("クリップボードに設定できません")
	}
	return nil
}

func windowProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	if taskbarCreated != 0 && message == taskbarCreated {
		app.addTrayIcon(app.iconData.Icon)
		return 0
	}
	switch message {
	case wmTray:
		if lparam == 0x0205 || lparam == 0x0204 {
			app.showMenu()
		} else if lparam == 0x0203 {
			app.showPopup()
		}
		return 0
	case wmHotkey:
		app.showPopup()
		return 0
	case wmCommand:
		app.handleMenuCommand(loword(wparam))
		return 0
	case wmSubmit:
		app.submit()
		return 0
	case wmTimer:
		if wparam == topmostTimerID && app.popup != 0 {
			procSetWindowPos.Call(app.popup, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow|swpNoActivate)
		}
		return 0
	case wmTranslateDone:
		result := <-app.resultCh
		if result.RequestID != app.requestID {
			return 0
		}
		if app.requestCancel != nil {
			app.requestCancel()
			app.requestCancel = nil
		}
		if result.Err != "" {
			app.busy = false
			app.logError("translation", fmt.Errorf("%s", result.Err))
			app.setPopupHint("翻訳に失敗しました。内容を確認して再試行してください")
			return 0
		}
		if err := copyClipboard(result.Text); err != nil {
			app.busy = false
			app.logError("clipboard", err)
			app.setPopupHint("クリップボードへコピーできませんでした")
			return 0
		}
		app.hidePopup()
		return 0
	case wmDestroy:
		if app.requestCancel != nil {
			app.requestCancel()
		}
		procUnregisterHK.Call(hwnd, 1)
		procShellNotify.Call(2, uintptr(unsafe.Pointer(&app.iconData)))
		procPostQuit.Call(0)
		return 0
	case wmClose:
		procShowWindow.Call(hwnd, swHide)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wparam, lparam)
	return r
}

func editProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	if message == wmKeyDown || message == 0x0104 { // WM_KEYDOWN / WM_SYSKEYDOWN
		if wparam == 0x0d {
			procPostMessage.Call(app.hwnd, wmSubmit, 0, 0)
			return 0
		}
		if wparam == 0x1b {
			app.hidePopup()
			return 0
		}
	}
	r, _, _ := procCallWindowProc.Call(oldEditProc, hwnd, uintptr(message), wparam, lparam)
	return r
}

// Keep imports/types explicit so the file remains self-contained and easy to audit.
var _ = rect{}
var _ = msgfClose
var _ = wmUser
var _ = editWantReturn
var _ = procGetClientRect
var _ = procSetWindowText
var _ = hiword
