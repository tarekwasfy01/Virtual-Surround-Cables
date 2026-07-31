param(
    [Parameter(Mandatory = $true)][string]$Png,
    [Parameter(Mandatory = $true)][string]$Ico
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$sizes = @(16, 20, 24, 32, 40, 48, 64, 128, 256)
$source = [System.Drawing.Image]::FromFile($Png)
$frames = [System.Collections.Generic.List[byte[]]]::new()

try {
    foreach ($size in $sizes) {
        $canvas = [System.Drawing.Bitmap]::new($size, $size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
        $graphics = [System.Drawing.Graphics]::FromImage($canvas)
        $stream = [System.IO.MemoryStream]::new()
        try {
            $graphics.Clear([System.Drawing.Color]::Transparent)
            $graphics.CompositingMode = [System.Drawing.Drawing2D.CompositingMode]::SourceCopy
            $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
            $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
            $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
            $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
            $scale = [Math]::Min($size / $source.Width, $size / $source.Height)
            $width = [Math]::Max(1, [int][Math]::Round($source.Width * $scale))
            $height = [Math]::Max(1, [int][Math]::Round($source.Height * $scale))
            $x = [int][Math]::Floor(($size - $width) / 2.0)
            $y = [int][Math]::Floor(($size - $height) / 2.0)
            $graphics.DrawImage($source, $x, $y, $width, $height)
            $canvas.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
            $frames.Add($stream.ToArray())
        } finally {
            $stream.Dispose(); $graphics.Dispose(); $canvas.Dispose()
        }
    }

    $directory = Split-Path -Parent $Ico
    if ($directory) { New-Item -ItemType Directory -Path $directory -Force | Out-Null }
    $file = [System.IO.File]::Open($Ico, [System.IO.FileMode]::Create)
    $writer = [System.IO.BinaryWriter]::new($file)
    try {
        $writer.Write([uint16]0); $writer.Write([uint16]1); $writer.Write([uint16]$sizes.Count)
        $offset = 6 + 16 * $sizes.Count
        for ($i = 0; $i -lt $sizes.Count; $i++) {
            $encodedSize = if ($sizes[$i] -eq 256) { 0 } else { $sizes[$i] }
            $writer.Write([byte]$encodedSize); $writer.Write([byte]$encodedSize)
            $writer.Write([byte]0); $writer.Write([byte]0)
            $writer.Write([uint16]1); $writer.Write([uint16]32)
            $writer.Write([uint32]$frames[$i].Length); $writer.Write([uint32]$offset)
            $offset += $frames[$i].Length
        }
        foreach ($frame in $frames) { $writer.Write($frame) }
    } finally { $writer.Dispose() }
    Write-Host "Created multi-resolution ICO: $Ico ($($sizes -join ', ') px)"
} finally { $source.Dispose() }
