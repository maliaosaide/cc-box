Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$cs = @'
using System;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;

public static class Cap {
    [DllImport("user32.dll")] public static extern bool EnumWindows(EWP f, IntPtr l);
    [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr h, StringBuilder t, int n);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int cmd);
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint f, uint x, uint y, uint d, int e);
    public delegate bool EWP(IntPtr h, IntPtr l);
    public const uint LDOWN = 0x02, LUP = 0x04;
    public const int SW_RESTORE = 9;

    public static IntPtr FindWindowByTitle(string sub) {
        IntPtr found = IntPtr.Zero;
        EnumWindows((h, l) => {
            if (IsWindowVisible(h)) {
                var sb = new StringBuilder(256);
                GetWindowText(h, sb, 256);
                if (sb.ToString().Contains(sub)) { found = h; return false; }
            }
            return true;
        }, IntPtr.Zero);
        return found;
    }
}
public struct RECT { public int L, T, R, B; }
'@
Add-Type -TypeDefinition $cs

$app = "C:/Users/a/Desktop/项目开发/cc-box/gui/build/bin/cc-box-gui.exe"

# Kill existing
Get-Process -Name "cc-box-gui" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 500

# Launch
$proc = Start-Process -FilePath $app -PassThru
Write-Output "Launched PID: $($proc.Id)"

# Wait for window (up to 15s)
$hwnd = [IntPtr]::Zero
for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Milliseconds 500
    $hwnd = [Cap]::FindWindowByTitle("CC-Box")
    if ($hwnd -ne [IntPtr]::Zero) { break }
}
if ($hwnd -eq [IntPtr]::Zero) {
    Write-Output "ERROR: Window not found after 15s"
    exit 1
}
Write-Output "Found window"

# Bring to front and restore if minimized
[Cap]::ShowWindow($hwnd, [Cap]::SW_RESTORE)
Start-Sleep -Milliseconds 500
[Cap]::SetForegroundWindow($hwnd)
Start-Sleep -Milliseconds 1000

$outDir = [Environment]::GetFolderPath("Desktop") + "\cc-box-shots"
mkdir $outDir -Force | Out-Null

function snap($name) {
    $r = New-Object RECT
    [Cap]::GetWindowRect($hwnd, [ref]$r)
    [Cap]::SetForegroundWindow($hwnd)
    Start-Sleep -Milliseconds 300
    $w = $r.R - $r.L; $h = $r.B - $r.T
    if ($w -lt 100 -or $h -lt 100) { Write-Output "WARN: window too small ($w x $h)"; return }
    $bmp = New-Object Drawing.Bitmap $w, $h
    $g = [Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($r.L, $r.T, 0, 0, (New-Object Drawing.Size $w, $h))
    $path = "$outDir\$name.png"
    $bmp.Save($path, [Drawing.Imaging.ImageFormat]::Png)
    Write-Output "OK: $name ($w x $h)"
    $g.Dispose(); $bmp.Dispose()
}

function click($x, $y) {
    $r = New-Object RECT
    [Cap]::GetWindowRect($hwnd, [ref]$r)
    $ax = $r.L + $x; $ay = $r.T + $y
    [Cap]::SetCursorPos($ax, $ay)
    Start-Sleep -Milliseconds 60
    [Cap]::mouse_event([Cap]::LDOWN, 0, 0, 0, 0)
    Start-Sleep -Milliseconds 60
    [Cap]::mouse_event([Cap]::LUP, 0, 0, 0, 0)
    Start-Sleep -Milliseconds 800
}

snap "01-dashboard"

# Sidebar Y positions depend on layout. Let's try clicking sidebar buttons.
# Each sidebar item is a button. We'll click the text area (x~120 left of window).
# Approximate Y positions (increased from top):
# Dashboard ~240, Files ~300, Binaries ~360, History ~480, Settings ~540
# These are guesses based on the sidebar layout in Sidebar.svelte

click 120 300; snap "02-files"
click 120 360; snap "03-binaries"
click 120 480; snap "04-history"
click 120 540; snap "05-settings"

Write-Output "Done. Files in: $outDir"
