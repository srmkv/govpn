package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
    "fyne.io/fyne/v2/theme"
    "runtime"
	"gopkg.in/ini.v1"
    
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"

	"fyne.io/fyne/v2/widget"
)

var tunnelNames []string
var selectedTunnel string
var logsOutput *widget.Entry
var isConnected bool

var statusLabel, pubkeyLabel, portLabel, addrLabel, dnsLabel *widget.Label
var peerPubkeyLabel, peerAllowedLabel, peerEndpointLabel, peerHandshakeLabel, peerTransferLabel *widget.Label
var toggleBtn *widget.Button

var tunnelListContainer *fyne.Container

func main() {
	a := app.New()
	w := a.NewWindow("NGFW_VPN")
    _ = os.MkdirAll(getConfigDir(), 0755)

	icon, err := fyne.LoadResourceFromPath("/usr/share/icons/hicolor/256x256/apps/wireguird.png")
	if err == nil {
		a.SetIcon(icon)
		w.SetIcon(icon)
	}

	w.Resize(fyne.NewSize(800, 500))

	logsOutput = widget.NewMultiLineEntry()
	logsOutput.SetPlaceHolder("Здесь появятся логи")
	logsOutput.Wrapping = fyne.TextWrapWord

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

	toggleBtn = widget.NewButton("Подключить", func() {
	if selectedTunnel == "" {
		appendLog("Выберите туннель.")
		return
	}
	name := strings.TrimSuffix(selectedTunnel, ".conf")

	var checkCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		checkCmd = exec.Command("sc", "query", name)
	} else {
		checkCmd = exec.Command("sudo", "wg", "show", name)
	}
	checkCmd.Stdin = strings.NewReader("")
	var checkOut bytes.Buffer
	checkCmd.Stdout = &checkOut
	checkCmd.Stderr = &checkOut
	checkCmd.Run()

	if runtime.GOOS == "windows" {
	checkCmd = exec.Command("sc", "query", "WireGuardTunnel$"+name)
} else {
		isConnected = strings.Contains(checkOut.String(), "interface: "+name)
	}

	var cmd *exec.Cmd
	if isConnected {
	if runtime.GOOS == "windows" {
		serviceName := "WireGuardTunnel$" + name
		_ = exec.Command("sc", "stop", serviceName).Run()

		// Удаляем туннель по имени (без .conf)
		cmd = exec.Command("wireguard.exe", "/uninstalltunnelservice", name)
	} else {
		cmd = exec.Command("sudo", "wg-quick", "down", name)
	}
} else {
	if runtime.GOOS == "windows" {
		cmd = exec.Command("wireguard.exe", "/installtunnelservice", filepath.Join(getConfigDir(), name+".conf"))
	} else {
		cmd = exec.Command("sudo", "wg-quick", "up", name)
	}
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
        outBytes, err := cmd.CombinedOutput()
        if err != nil {
            appendLog("Ошибка добавления:\n" + string(outBytes))
            return
        }

        tunnelNames = append(tunnelNames, name)
        refreshTunnelList(w)
        appendLog("Добавлен: " + name)
    }, w)
})


	interfaceBox := container.NewVBox(
		statusLabel, pubkeyLabel, portLabel, addrLabel, dnsLabel, toggleBtn,
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

func refreshTunnelList(w fyne.Window) {
	tunnelListContainer.Objects = nil

	for _, name := range tunnelNames {
		confName := name

		label := widget.NewLabel(confName)
		label.Alignment = fyne.TextAlignLeading

		// Кнопка редактирования
		editBtn := widget.NewButton("✏️", func() {
			editConfigDialog(w, confName)
		})
		editBtn.Importance = widget.LowImportance

		// Кнопка удаления
	removeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
	dialog.ShowConfirm("Удалить конфигурацию", "Удалить "+confName+"?", func(confirm bool) {
		if confirm {
			var cmd *exec.Cmd
if runtime.GOOS == "windows" {
	_ = exec.Command("wireguard.exe", "/uninstalltunnelservice", filepath.Join(getConfigDir(), confName)).Run()
	cmd = exec.Command("cmd", "/C", "del", filepath.Join(getConfigDir(), confName))
} else {
	cmd = exec.Command("sudo", "rm", "/etc/wireguard/"+confName)
}

			cmd.Stdin = strings.NewReader("") // не блокируем ожидание ввода
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out
			err := cmd.Run()
			if err != nil {
				appendLog("Ошибка удаления:\n" + out.String())
			} else {
				appendLog("Удалено: " + confName)
				loadTunnels()
				refreshTunnelList(w)
			}
		}
	}, w)
})


		removeBtn.Importance = widget.DangerImportance

		// Горизонтальный блок: имя + иконки
		row := container.NewBorder(nil, nil, nil,
			container.NewHBox(editBtn, removeBtn),
			label,
		)

		// Кнопка-слой на всю строку для выбора
		selectBtn := widget.NewButton("", func() {
			selectedTunnel = confName
			
			confPath := filepath.Join(getConfigDir(), confName)

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
		})
		selectBtn.Importance = widget.MediumImportance
		selectBtn.SetText("") // пустой текст

		rowContainer := container.NewMax(selectBtn, row)
		tunnelListContainer.Add(rowContainer)
	}

	tunnelListContainer.Refresh()
}



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
		tmp := "/tmp/" + filename
		_ = os.WriteFile(tmp, []byte(entry.Text), 0600)

		cmd := exec.Command("sudo", "cp", tmp, path)
		cmd.Stdin = strings.NewReader("")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
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

func getConfigDir() string {
	if fyne.CurrentDevice().IsMobile() {
		return "" // не используется
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "WireGuird")
	}
	return "/etc/wireguard"
}
