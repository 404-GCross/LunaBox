<#
.SYNOPSIS
  把指定进程的主窗口调整到固定大小并截图（PrintWindow，窗口被遮挡也能截），用于和 Wails 版逐像素对比。

.EXAMPLE
  pwsh scripts/capture-window.ps1 -Name LunaBox -Out wails.png
  pwsh scripts/capture-window.ps1 -Name desktop-mygo -Out mygo.png -Width 1920 -Height 1200
#>
param(
  [Parameter(Mandatory = $true)][string]$Name,
  [Parameter(Mandatory = $true)][string]$Out,
  [int]$Width = 1920,
  [int]$Height = 1200,
  [int]$SettleMs = 1500
)

Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class Win {
  [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr h, IntPtr after, int x, int y, int cx, int cy, uint flags);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr hdc, uint flags);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int cmd);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
}
"@

$proc = Get-Process -Name $Name -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1
if (-not $proc) { throw "no window for process '$Name'" }
$h = $proc.MainWindowHandle

[Win]::ShowWindow($h, 9) | Out-Null # SW_RESTORE
[Win]::SetWindowPos($h, [IntPtr]::Zero, 40, 40, $Width, $Height, 0x0004) | Out-Null # SWP_NOZORDER
Start-Sleep -Milliseconds $SettleMs

$r = New-Object Win+RECT
[Win]::GetWindowRect($h, [ref]$r) | Out-Null
$w = $r.Right - $r.Left
$hgt = $r.Bottom - $r.Top
$bmp = New-Object System.Drawing.Bitmap $w, $hgt
$g = [System.Drawing.Graphics]::FromImage($bmp)
$hdc = $g.GetHdc()
[Win]::PrintWindow($h, $hdc, 2) | Out-Null # PW_RENDERFULLCONTENT
$g.ReleaseHdc($hdc)
$g.Dispose()
$bmp.Save($Out, [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()
"saved {0} ({1}x{2})" -f $Out, $w, $hgt
