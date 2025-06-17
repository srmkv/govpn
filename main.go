package main

import (
    "bytes"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/widget"
)

var selectedConfName string
var tempFilePath string

func main() {
    a := app.New()
    w := a.NewWindow("WireGuard GUI")
    w.Resize(fyne.NewSize(600, 400))

    output := widget.NewMultiLineEntry()
    output.SetPlaceHolder("Здесь появится содержимое .conf и результат запуска wg-quick")

    openBtn := widget.NewButton("Загрузить конфиг", func() {
        dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
            if err == nil && reader != nil {
                data, _ := io.ReadAll(reader)
                selectedConfName = filepath.Base(reader.URI().Path())
                tempFilePath = "/etc/wireguard/" + selectedConfName

                tmpPath := "/tmp/" + selectedConfName
                os.WriteFile(tmpPath, data, 0600)

                // Копируем напрямую (предполагаем, что приложение запущено от root)
                cmd := exec.Command("sudo", "cp", tmpPath, tempFilePath)
                var out bytes.Buffer
                cmd.Stdout = &out
                cmd.Stderr = &out
                err := cmd.Run()
                if err != nil {
                    output.SetText("Ошибка копирования:\n" + out.String())
                    return
                }
                output.SetText(string(data))
            }
        }, w)
    })

    startBtn := widget.NewButton("Включить VPN", func() {
        if selectedConfName == "" {
            output.SetText("Сначала загрузите конфигурацию.")
            return
        }
        name := strings.TrimSuffix(selectedConfName, filepath.Ext(selectedConfName))
        cmd := exec.Command("sudo", "wg-quick", "up", name)
        cmd.Stdin = strings.NewReader("")
        var out bytes.Buffer
        cmd.Stdout = &out
        cmd.Stderr = &out
        err := cmd.Run()
        if err != nil {
            output.SetText("Ошибка запуска:\n" + out.String())
        } else {
            output.SetText("Подключено:\n" + out.String())
        }
    })

    stopBtn := widget.NewButton("Выключить VPN", func() {
        if selectedConfName == "" {
            output.SetText("Сначала загрузите конфигурацию.")
            return
        }
        name := strings.TrimSuffix(selectedConfName, filepath.Ext(selectedConfName))
        cmd := exec.Command("sudo", "wg-quick", "down", name)
        cmd.Stdin = strings.NewReader("")
        var out bytes.Buffer
        cmd.Stdout = &out
        cmd.Stderr = &out
        err := cmd.Run()
        if err != nil {
            output.SetText("Ошибка отключения:\n" + out.String())
        } else {
            output.SetText("Отключено:\n" + out.String())
        }
    })

    content := container.NewVBox(
        openBtn,
        startBtn,
        stopBtn,
        output,
    )

    w.SetContent(content)
    w.ShowAndRun()
}
