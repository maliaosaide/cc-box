package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFrontendClickTargetsUseNativeControls(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"frontend/src/pages/Projects.svelte", `<button class="proj-row" type="button"`},
		{"frontend/src/pages/History.svelte", `<button class="snap-row" type="button"`},
		{"frontend/src/lib/components/TreeNode.svelte", `<button class="tree-dir" type="button"`},
		{"frontend/src/lib/components/TreeNode.svelte", `<button class="tree-file" class:selected={selectedPath === node.path} type="button"`},
		{"frontend/src/pages/Files.svelte", `role="dialog" aria-modal="true"`},
	}

	for _, tc := range cases {
		t.Run(tc.file+tc.want, func(t *testing.T) {
			data, err := os.ReadFile(filepath.FromSlash(tc.file))
			if err != nil {
				t.Fatalf("读取 %s 失败: %v", tc.file, err)
			}
			if !strings.Contains(string(data), tc.want) {
				t.Fatalf("%s 缺少原生交互标记 %q", tc.file, tc.want)
			}
		})
	}
}

func TestSettingsLabelsAreAssociated(t *testing.T) {
	data, err := os.ReadFile(filepath.FromSlash("frontend/src/pages/Settings.svelte"))
	if err != nil {
		t.Fatalf("读取 Settings.svelte 失败: %v", err)
	}
	if strings.Contains(string(data), `<label class="label">`) {
		t.Fatal("Settings.svelte 仍存在未绑定控件的 label")
	}
}

func TestDesktopNativeClickSmoke(t *testing.T) {
	if os.Getenv("CC_BOX_DESKTOP_UI_SMOKE") != "1" {
		t.Skip("设置 CC_BOX_DESKTOP_UI_SMOKE=1 和 CC_BOX_DESKTOP_EXE 后运行真实桌面点击 smoke 测试")
	}
	if runtime.GOOS != "windows" {
		t.Skip("真实桌面点击 smoke 测试仅支持 Windows UIAutomation")
	}

	exe := os.Getenv("CC_BOX_DESKTOP_EXE")
	if exe == "" {
		t.Fatal("缺少 CC_BOX_DESKTOP_EXE")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("CC_BOX_DESKTOP_EXE 不可用: %v", err)
	}

	home := t.TempDir()
	seedDesktopSmokeHome(t, home)

	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "CC_BOX_WEBDAV_PASSWORD=smoke")
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动桌面程序失败: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
	})

	ps := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", desktopNativeClickScript())
	out, err := ps.CombinedOutput()
	if err != nil {
		t.Fatalf("桌面原生点击 smoke 测试失败: %v\n%s", err, out)
	}
}

func seedDesktopSmokeHome(t *testing.T, home string) {
	t.Helper()

	mustMkdir := func(path string) {
		t.Helper()
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatalf("创建目录 %s 失败: %v", path, err)
		}
	}

	mustMkdir(filepath.Join(home, ".cc-box"))
	mustMkdir(filepath.Join(home, ".claude"))
	mustMkdir(filepath.Join(home, ".local", "bin"))
	mustMkdir(filepath.Join(home, ".local", "share", "claude", "versions"))

	cfg := `[webdav]
url = ""
username = ""
root = "/cc-box/"

[encryption]
enabled = false

[sync]
snapshot_limit = 50
conflict_strategy = "ask"
merge_retry_max = 3
auto_sync_interval = ""

[device]
id = "desktop-smoke"
name = "Desktop Smoke"

[claude]
path = ""

[binary]
encrypt = false
chunk_mode = "auto"
chunk_size_mb = 10
chunk_threshold_mb = 50
auto_upload = false
bin_dir = ""
versions_dir = ""

[exclude]
patterns = ["sessions/", "cache/", "*.lock"]
`
	if err := os.WriteFile(filepath.Join(home, ".cc-box", "config.toml"), []byte(cfg), 0600); err != nil {
		t.Fatalf("写入桌面 smoke 配置失败: %v", err)
	}
}

func desktopNativeClickScript() string {
	return `$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

function Find-CCBoxWindow {
    $nameCond = New-Object System.Windows.Automation.PropertyCondition -ArgumentList ([System.Windows.Automation.AutomationElement]::NameProperty), 'CC-Box'
    for ($i = 0; $i -lt 80; $i++) {
        $root = [System.Windows.Automation.AutomationElement]::RootElement
        $window = $root.FindFirst([System.Windows.Automation.TreeScope]::Children, $nameCond)
        if ($null -ne $window) { return $window }
        Start-Sleep -Milliseconds 250
    }
    throw '未找到 CC-Box 窗口'
}

function Invoke-ButtonByName($window, $name) {
    $nameCond = New-Object System.Windows.Automation.PropertyCondition -ArgumentList ([System.Windows.Automation.AutomationElement]::NameProperty), $name
    $typeCond = New-Object System.Windows.Automation.PropertyCondition -ArgumentList ([System.Windows.Automation.AutomationElement]::ControlTypeProperty), ([System.Windows.Automation.ControlType]::Button)
    $cond = New-Object System.Windows.Automation.AndCondition -ArgumentList $nameCond, $typeCond
    for ($i = 0; $i -lt 40; $i++) {
        $button = $window.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond)
        if ($null -ne $button) {
            $window.SetFocus()
            $invoke = $button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
            $invoke.Invoke()
            Start-Sleep -Milliseconds 300
            return
        }
        Start-Sleep -Milliseconds 250
    }
    throw "未找到按钮: $name"
}

$window = Find-CCBoxWindow
foreach ($name in @('配置', '二进制', '项目', '历史', '设置', '概览')) {
    Invoke-ButtonByName $window $name
}
Write-Output 'desktop native click smoke passed'
`
}
