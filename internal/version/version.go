package version

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"

	"github.com/ed/stead/internal/ui"
)

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

const banner = "███████╗████████╗███████╗ █████╗ ██████╗ \n" +
	"██╔════╝╚══██╔══╝██╔════╝██╔══██╗██╔══██╗\n" +
	"███████╗   ██║   █████╗  ███████║██║  ██║\n" +
	"╚════██║   ██║   ██╔══╝  ██╔══██║██║  ██║\n" +
	"███████║   ██║   ███████╗██║  ██║██████╔╝\n" +
	"╚══════╝   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═════╝ "

func Print(out io.Writer) {
	if out == nil {
		return
	}
	info := Info()
	printBanner(out)
	fmt.Fprintln(out)
	ui.PrintTitle(out, "Stead version")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Version", info.Version)
	ui.PrintKV(out, "Commit", info.Commit)
	ui.PrintKV(out, "Build date", info.Date)
	ui.PrintKV(out, "Go", runtime.Version())
	ui.PrintKV(out, "OS/Arch", runtime.GOOS+"/"+runtime.GOARCH)
}

func printBanner(out io.Writer) {
	fmt.Fprintln(out, ui.Title(out, banner))
	fmt.Fprintln(out, ui.Detail(out, "normal OpenSSH remote dev; Tailscale SSH is not used"))
}

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func Info() BuildInfo {
	info := BuildInfo{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
	}
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "unknown" && setting.Value != "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.Date == "unknown" && setting.Value != "" {
					info.Date = setting.Value
				}
			case "vcs.modified":
				if setting.Value == "true" && info.Commit != "unknown" {
					info.Commit += " (modified)"
				}
			}
		}
	}
	return info
}
