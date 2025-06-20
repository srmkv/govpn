package main

import (
	"bytes"
	"image/color"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gopkg.in/ini.v1"
)

// -------------------
// ColorButton widget
// -------------------

type ColorButton struct {
	widget.BaseWidget
	Text    string
	BgColor color.Color
	OnTap   func()
}

func NewColorButton(text string, bg color.Color, tap func()) *ColorButton {
	btn := &ColorButton{Text: text, BgColor: bg, OnTap: tap}
	btn.ExtendBaseWidget(btn)
	return btn
}

func (c *ColorButton) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(c.BgColor)
	label := canvas.NewText(c.Text, color.White)
	label.Alignment = fyne.TextAlignCenter
	label.TextSize = 16
	objects := []fyne.CanvasObject{rect, label}
	return &colorButtonRenderer{btn: c, rect: rect, label: label, objects: objects}
}

type colorButtonRenderer struct {
	btn     *ColorButton
	rect    *canvas.Rectangle
	label   *canvas.Text
	objects []fyne.CanvasObject
}

func (r *colorButtonRenderer) Layout(size fyne.Size) {
	r.rect.Resize(size)
	r.label.Resize(fyne.NewSize(size.Width, r.label.MinSize().Height))
	r.label.Move(fyne.NewPos(0, (size.Height-r.label.MinSize().Height)/2))
}

func (r *colorButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSize(120, 40)
}

func (r *colorButtonRenderer) Refresh() {
	r.rect.FillColor = r.btn.BgColor
	r.label.Text = r.btn.Text
	r.rect.Refresh()
	r.label.Refresh()
}

func (r *colorButtonRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *colorButtonRenderer) Destroy()                     {}

func (c *ColorButton) Tapped(_ *fyne.PointEvent) {
	if c.OnTap != nil {
		c.OnTap()
	}
}

// -------------------
// Global variables
// -------------------

var (
	tunnelNames          []string
	selectedTunnel       string
	logsOutput           *widget.Entry
	isConnected          bool
	statusLabel          *widget.Label
	pubkeyLabel          *widget.Label
	portLabel            *widget.Label
	addrLabel            *widget.Label
	dnsLabel             *widget.Label
	peerPubkeyLabel      *widget.Label
	peerAllowedLabel     *widget.Label
	peerEndpointLabel    *widget.Label
	peerHandshakeLabel   *widget.Label
	peerTransferLabel    *widget.Label
	tunnelListContainer  *fyne.Container
	toggleBtn            *ColorButton
)

// -------------------
// main()
// -------------------

func main() {
	a := app.New()
	w := a.NewWindow("NGFW_VPN")
	_ = os.MkdirAll(getConfigDir(), 0755)

	// Загрузка иконки
	if icon, err := fyne.LoadResourceFromPath("/usr/share/icons/hicolor/256x256/apps/wireguird.png"); err == nil {
		a.SetIcon(icon)
		w.SetIcon(icon)
	}

	w.Resize(fyne.NewSize(800, 500))

	// Периодически проверять статус
	go func() {
		for {
			time.Sleep(5 * time.Second)
			checkStatusAndUpdateUI()
		}
	}()

	// Логи
	logsOutput = widget.NewMultiLineEntry()
	logsOutput.SetPlaceHolder("Здесь появятся логи")
	logsOutput.Wrapping = fyne.TextWrapWord

	// Метки статуса
	statusLabel = widget.NewLabel("Status: unknown")
	pubkeyLabel = widget.NewLabel("Public key: unknown")
	portLabel = widget.NewLabel("Listen port: unknown")
	addrLabel = widget.NewLabel("Addresses: unknown")
	dnsLabel = widget.NewLabel("DNS servers: unknown")

	peerPubkeyLabel = widget.NewLabel("Public key: unknown")
	peerAllowedLabel = widget.NewLabel("Allowed IPs: unknown")
	peerEndpointLabel = widget.NewLabel("Endpoint: unknown")
	peerHandshakeLabel = widget.NewLabel("Latest handshake: unknown")
	peerTransferLabel = widget.NewLabel("Transfer: unknown")

	// Кнопка подключения/отключения
	toggleBtn = NewColorButton("Подключить", color.RGBA{0x1e, 0x1d, 0x85, 0xff}, toggleAction)
	updateToggleButton()

	// Кнопка "Обновить"
	refreshBtn := widget.NewButton("🔄", checkStatusAndUpdateUI)

	// Кнопка "Добавить туннель"
	addBtn := widget.NewButton("Добавить туннель", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			data, _ := io.ReadAll(reader)
			name := filepath.Base(reader.URI().Path())
			path := filepath.Join(getConfigDir(), name)
			tmp := filepath.Join(os.TempDir(), name)
			_ = os.WriteFile(tmp, data, 0600)

			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.Command("cmd", "/C", "copy", "/Y", tmp, path)
			} else {
				cmd = exec.Command("bash", "-c", "sudo cp '"+tmp+"' '"+path+"' && sudo chmod o+r '"+path+"'")
			}
			cmd.Stdin = strings.NewReader("")
			if out, err := cmd.CombinedOutput(); err != nil {
				appendLog("Ошибка добавления:\n" + string(out))
				return
			}

			tunnelNames = append(tunnelNames, name)
			refreshTunnelList(w)
			appendLog("Добавлен: " + name)
		}, w)
	})

	// Компоновка интерфейса
	interfaceBox := container.NewVBox(
		statusLabel, pubkeyLabel, portLabel, addrLabel, dnsLabel,
		container.NewHBox(toggleBtn, refreshBtn),
	)
	peerBox := container.NewVBox(
		peerPubkeyLabel, peerAllowedLabel, peerEndpointLabel, peerHandshakeLabel, peerTransferLabel,
	)
	configContent := container.NewVBox(interfaceBox, widget.NewSeparator(), peerBox)

	tunnelListContainer = container.NewVBox()
	left := container.NewBorder(nil, addBtn, nil, nil, container.NewVScroll(tunnelListContainer))

	split := container.NewHSplit(left, configContent)
	split.Offset = 0.25

	tabs := container.NewAppTabs(
		container.NewTabItem("Конфигурации", split),
		container.NewTabItem("Логи", logsOutput),
	)
	w.SetContent(tabs)

	loadTunnels()
	refreshTunnelList(w)
	w.ShowAndRun()
}

// -------------------
// toggleAction()
// -------------------

// toggleAction — подключение/отключение туннеля
func toggleAction() {
	if selectedTunnel == "" {
		appendLog("Выберите туннель.")
		return
	}
	name := strings.TrimSuffix(selectedTunnel, ".conf")
	confPath := filepath.Join(getConfigDir(), name+".conf")

	var checkCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		checkCmd = exec.Command("sc", "query", "WireGuardTunnel$"+name)
	} else {
		checkCmd = exec.Command("sudo", "wg", "show", name)
	}
	var buf bytes.Buffer
	checkCmd.Stdout, checkCmd.Stderr = &buf, &buf
	checkCmd.Run()
	isConnected = strings.Contains(buf.String(), name)

	var cmd *exec.Cmd
	if isConnected {
		// используем путь — чтобы точно отключался тот конфиг
		cmd = exec.Command("sudo", "-S", "wg-quick", "down", confPath)
	} else {
		_ = os.Chmod(confPath, 0600) // убираем warning
		cmd = exec.Command("sudo", "-S", "wg-quick", "up", confPath)
	}

	cmd.Stdin = strings.NewReader("")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err != nil {
		action := map[bool]string{true: "отключения", false: "подключения"}[isConnected]
		appendLog("Ошибка " + action + ":\n" + out.String())
		return
	}

	isConnected = !isConnected
	status := map[bool]string{true: "Подключено", false: "Отключено"}[isConnected]
	statusLabel.SetText("Status: " + status)
	appendLog(status + ":\n" + out.String())
	updateToggleButton()

	// 🔄 ОБНОВЛЯЕМ СПИСОК ДЛЯ ОБНОВЛЕНИЯ ЦВЕТА КРУЖКА
	refreshTunnelList(fyne.CurrentApp().Driver().AllWindows()[0])
}




// -------------------
// updateToggleButton()
// -------------------

func updateToggleButton() {
	if isConnected {
		toggleBtn.Text = "Отключить"
		toggleBtn.BgColor = color.RGBA{0xc7, 0x22, 0x1a, 0xff}
	} else {
		toggleBtn.Text = "Подключить"
		toggleBtn.BgColor = color.RGBA{0x28, 0xa7, 0x45, 0xff}
	}
	toggleBtn.Refresh()
}

// -------------------
// loadTunnels()
// -------------------

func loadTunnels() {
	tunnelNames = nil
	files, err := os.ReadDir(getConfigDir())
	if err != nil {
		appendLog("Ошибка чтения каталога: " + err.Error())
		return
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".conf") {
			tunnelNames = append(tunnelNames, f.Name())
		}
	}
}

// -------------------
// refreshTunnelList()
// -------------------

func refreshTunnelList(w fyne.Window) {
	tunnelListContainer.Objects = nil
	for _, name := range tunnelNames {
		confName := name

		// Проверка статуса туннеля
		
		// Проверка статуса туннеля
var statusDot *canvas.Text
if isTunnelConnected(confName) {
	statusDot = canvas.NewText("●", color.RGBA{0x28, 0xa7, 0x45, 0xff}) // зелёный
} else {
	statusDot = canvas.NewText("●", color.Gray{Y: 160}) // серый
}
statusDot.TextSize = 14


		// Имя конфигурации рядом с кружком
		nameWithDot := container.NewHBox(statusDot, widget.NewLabel(" "), widget.NewLabel(confName))



		editBtn := widget.NewButton("✏️", func() {
			editConfigDialog(w, confName)
		})
		editBtn.Importance = widget.LowImportance

		removeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
			dialog.ShowConfirm("Удалить конфигурацию", "Удалить "+confName+"?", func(confirm bool) {
				if !confirm {
					return
				}
				var cmd *exec.Cmd
				if runtime.GOOS == "windows" {
					_ = exec.Command("wireguard.exe", "/uninstalltunnelservice", strings.TrimSuffix(confName, ".conf")).Run()
					cmd = exec.Command("cmd", "/C", "del", filepath.Join(getConfigDir(), confName))
				} else {
					cmd = exec.Command("sudo", "rm", filepath.Join(getConfigDir(), confName))
				}
				cmd.Stdin = strings.NewReader("")
				var out bytes.Buffer
				cmd.Stdout, cmd.Stderr = &out, &out
				if err := cmd.Run(); err != nil {
					appendLog("Ошибка удаления:\n" + out.String())
				} else {
					appendLog("Удалено: " + confName)
					loadTunnels()
					refreshTunnelList(w)
				}
			}, w)
		})
		removeBtn.Importance = widget.DangerImportance

		row := container.NewBorder(nil, nil, nil, container.NewHBox(editBtn, removeBtn), nameWithDot)

		selectBtn := widget.NewButton("", func() {
			selectedTunnel = confName
			checkStatusAndUpdateUI()
			iface, peer, err := parseConfig(filepath.Join(getConfigDir(), confName))
			if err != nil {
				appendLog("Ошибка чтения конфига: " + err.Error())
				return
			}
			addrLabel.SetText("Addresses: " + iface["Address"])
			dnsLabel.SetText("DNS servers: " + iface["DNS"])
			peerPubkeyLabel.SetText("Public key: " + peer["PublicKey"])
			peerAllowedLabel.SetText("Allowed IPs: " + peer["AllowedIPs"])
			peerEndpointLabel.SetText("Endpoint: " + peer["Endpoint"])
			peerHandshakeLabel.SetText("Latest handshake: unknown")
			peerTransferLabel.SetText("Transfer: unknown")
		})
		selectBtn.Importance = widget.MediumImportance
		selectBtn.SetText("")

		rowContainer := container.NewMax(selectBtn, row)
		tunnelListContainer.Add(rowContainer)
	}
	tunnelListContainer.Refresh()
}


// -------------------
// editConfigDialog()
// -------------------

func editConfigDialog(w fyne.Window, filename string) {
	path := filepath.Join(getConfigDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		dialog.ShowError(err, w)
		return
	}
	entry := widget.NewMultiLineEntry()
	entry.SetText(string(data))

	var dlg dialog.Dialog
	saveBtn := widget.NewButton("Сохранить", func() {
		tmp := filepath.Join(os.TempDir(), filename)
		_ = os.WriteFile(tmp, []byte(entry.Text), 0600)
		cmd := exec.Command("sudo", "cp", tmp, path)
		cmd.Stdin = strings.NewReader("")
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			appendLog("Ошибка при сохранении:\n" + out.String())
			return
		}
		appendLog("Сохранено: " + filename)
		dlg.Hide()
	})

	content := container.NewBorder(nil, saveBtn, nil, nil, container.NewVScroll(entry))
	dlg = dialog.NewCustom("Редактировать "+filename, "Закрыть", content, w)
	dlg.Resize(fyne.NewSize(800, 600))
	dlg.Show()
}

// -------------------
// parseConfig()
// -------------------

func parseConfig(path string) (map[string]string, map[string]string, error) {
	cfg, err := ini.Load(path)
	if err != nil {
		return nil, nil, err
	}
	iface := map[string]string{
		"Address":    cfg.Section("Interface").Key("Address").String(),
		"DNS":        cfg.Section("Interface").Key("DNS").String(),
		"PrivateKey": cfg.Section("Interface").Key("PrivateKey").String(),
	}
	peer := map[string]string{
		"PublicKey":  cfg.Section("Peer").Key("PublicKey").String(),
		"AllowedIPs": cfg.Section("Peer").Key("AllowedIPs").String(),
		"Endpoint":   cfg.Section("Peer").Key("Endpoint").String(),
	}
	return iface, peer, nil
}

// -------------------
// appendLog()
// -------------------

func appendLog(msg string) {
	logsOutput.SetText(strings.TrimSpace(logsOutput.Text) + "\n" + msg)
}

// -------------------
// getConfigDir()
// -------------------

func getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// fallback
		return "/tmp/wireguird"
	}
	return filepath.Join(home, ".wireguird")
}

// -------------------
// checkStatusAndUpdateUI()
// -------------------

func checkStatusAndUpdateUI() {
	if selectedTunnel == "" {
		return
	}
	name := strings.TrimSuffix(selectedTunnel, ".conf")
	var checkCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		checkCmd = exec.Command("sc", "query", "WireGuardTunnel$"+name)
	} else {
		checkCmd = exec.Command("sudo", "wg", "show", name)
	}
	var out bytes.Buffer
	checkCmd.Stdout, checkCmd.Stderr = &out, &out
	checkCmd.Run()

	if runtime.GOOS == "windows" {
		isConnected = strings.Contains(out.String(), "RUNNING")
	} else {
		isConnected = strings.Contains(out.String(), "interface: "+name)
	}
	status := map[bool]string{true: "Подключено", false: "Отключено"}[isConnected]
	statusLabel.SetText("Status: " + status)
	updateToggleButton()
}
func isTunnelConnected(name string) bool {
	name = strings.TrimSuffix(name, ".conf")
	var checkCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		checkCmd = exec.Command("sc", "query", "WireGuardTunnel$"+name)
	} else {
		checkCmd = exec.Command("sudo", "wg", "show", name)
	}
	var out bytes.Buffer
	checkCmd.Stdout, checkCmd.Stderr = &out, &out
	_ = checkCmd.Run()

	if runtime.GOOS == "windows" {
		return strings.Contains(out.String(), "RUNNING")
	}
	return strings.Contains(out.String(), "interface: "+name)
}
