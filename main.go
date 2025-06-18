package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
    "fmt"
	"gopkg.in/ini.v1"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var tunnelList *widget.List
var tunnelNames []string
var selectedTunnel string
var logsOutput *widget.Entry
var isConnected bool

var statusLabel, pubkeyLabel, portLabel, addrLabel, dnsLabel *widget.Label
var peerPubkeyLabel, peerAllowedLabel, peerEndpointLabel, peerHandshakeLabel, peerTransferLabel *widget.Label
var toggleBtn *widget.Button

func main() {
	a := app.New()
w := a.NewWindow("NGFW_VPN")

icon, err := fyne.LoadResourceFromPath("/usr/share/icons/hicolor/256x256/apps/wireguird.png")
if err == nil {
	a.SetIcon(icon)
	w.SetIcon(icon)
}


	
	w.Resize(fyne.NewSize(800, 500))

	logsOutput = widget.NewMultiLineEntry()
	logsOutput.SetPlaceHolder("Здесь появятся логи")
	logsOutput.Wrapping = fyne.TextWrapWord

	// --- Интерфейсные лейблы ---
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

	// --- Список туннелей ---
	tunnelList = widget.NewList(
		func() int { return len(tunnelNames) },
		func() fyne.CanvasObject {
			return widget.NewLabel("Туннель")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(tunnelNames[i])
		},
	)

	tunnelList.OnSelected = func(id int) {
		selectedTunnel = tunnelNames[id]
		confPath := "/etc/wireguard/" + selectedTunnel
		iface, peer, err := parseConfig(confPath)
		if err != nil {
			appendLog("Ошибка чтения конфига: " + err.Error())
			return
		}

		statusLabel.SetText("Status: Inactive")
		pubkeyLabel.SetText("Public key: unknown")
		portLabel.SetText("Listen port: unknown")
		addrLabel.SetText("Addresses: " + iface["Address"])
		dnsLabel.SetText("DNS servers: " + iface["DNS"])

		peerPubkeyLabel.SetText("Public key: " + peer["PublicKey"])
		peerAllowedLabel.SetText("Allowed IPs: " + peer["AllowedIPs"])
		peerEndpointLabel.SetText("Endpoint: " + peer["Endpoint"])
		peerHandshakeLabel.SetText("Latest handshake: unknown")
		peerTransferLabel.SetText("Transfer: unknown")

		isConnected = false
		toggleBtn.SetText("Подключить")
	}

	// --- Кнопка "Добавить туннель" ---
	addBtn := widget.NewButton("Добавить туннель", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			data, _ := io.ReadAll(reader)
			name := filepath.Base(reader.URI().Path())
			path := "/etc/wireguard/" + name

			tmp := "/tmp/" + name
			_ = os.WriteFile(tmp, data, 0600)

			cmd := exec.Command("sudo", "cp", tmp, path)
			cmd.Stdin = strings.NewReader("")
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out
			if err := cmd.Run(); err != nil {
				appendLog("Ошибка добавления:\n" + out.String())
				return
			}

			tunnelNames = append(tunnelNames, name)
			tunnelList.Refresh()
			appendLog("Добавлен: " + name)
		}, w)
	})

	// --- Кнопка "Подключить/Отключить" ---
	toggleBtn = widget.NewButton("Подключить", func() {
	if selectedTunnel == "" {
		appendLog("Выберите туннель.")
		return
	}
	name := strings.TrimSuffix(selectedTunnel, ".conf")

	// Проверяем статус подключения
	checkCmd := exec.Command("sudo", "wg", "show", name)
	checkCmd.Stdin = strings.NewReader("")
	var checkOut bytes.Buffer
	checkCmd.Stdout = &checkOut
	checkCmd.Stderr = &checkOut
	checkCmd.Run()

	isConnected = strings.Contains(checkOut.String(), "interface: "+name)

	var cmd *exec.Cmd
	if isConnected {
		cmd = exec.Command("sudo", "wg-quick", "down", name)
	} else {
		cmd = exec.Command("sudo", "wg-quick", "up", name)
	}

	cmd.Stdin = strings.NewReader("")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	if err != nil {
		appendLog("Ошибка " + map[bool]string{true: "отключения", false: "подключения"}[isConnected] + ":\n" + out.String())
	} else {
		isConnected = !isConnected
		status := map[bool]string{true: "Подключено", false: "Отключено"}[isConnected]
		toggleBtn.SetText(map[bool]string{true: "Отключить", false: "Подключить"}[isConnected])
		statusLabel.SetText("Status: " + status)
		appendLog(status + ":\n" + out.String())
	}
})


	// --- Layout правой части ---
	interfaceBox := container.NewVBox(
		statusLabel, pubkeyLabel, portLabel, addrLabel, dnsLabel, toggleBtn,
	)
	peerBox := container.NewVBox(
		peerPubkeyLabel, peerAllowedLabel, peerEndpointLabel, peerHandshakeLabel, peerTransferLabel,
	)
	configContent := container.NewVBox(interfaceBox, widget.NewSeparator(), peerBox)

	// --- Левая колонка ---
	left := container.NewBorder(nil, addBtn, nil, nil, tunnelList)

	// --- Вкладки ---
	tabs := container.NewAppTabs(
		container.NewTabItem("Конфигурации", container.NewHSplit(left, configContent)),
		container.NewTabItem("Логи", logsOutput),
	)

	w.SetContent(tabs)

	// --- Загрузка туннелей ---
	loadTunnels()
	tunnelList.Refresh()
    fmt.Println("App icon:", a.Icon().Name())
fmt.Println("Window icon:", w.Icon().Name())

	w.ShowAndRun()
}

func loadTunnels() {
	tunnelNames = []string{}
	files, err := os.ReadDir("/etc/wireguard/")
	if err != nil {
		appendLog("Ошибка чтения каталога: " + err.Error())
		return
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".conf") {
			tunnelNames = append(tunnelNames, file.Name())
		}
	}
}

func parseConfig(path string) (map[string]string, map[string]string, error) {
	cfg, err := ini.Load(path)
	if err != nil {
		return nil, nil, err
	}

	iface := map[string]string{}
	peer := map[string]string{}

	ifaceSec := cfg.Section("Interface")
	peerSec := cfg.Section("Peer")

	iface["Address"] = ifaceSec.Key("Address").String()
	iface["DNS"] = ifaceSec.Key("DNS").String()
	iface["PrivateKey"] = ifaceSec.Key("PrivateKey").String()

	peer["PublicKey"] = peerSec.Key("PublicKey").String()
	peer["AllowedIPs"] = peerSec.Key("AllowedIPs").String()
	peer["Endpoint"] = peerSec.Key("Endpoint").String()

	return iface, peer, nil
}

func appendLog(msg string) {
	current := logsOutput.Text
	logsOutput.SetText(current + "\n" + msg)
}
