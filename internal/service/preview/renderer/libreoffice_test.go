package renderer

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestOfficeProfileURLWindowsDrivePath(t *testing.T) {
	// Windows 盘符路径（C:\...）的 file URI 编码是 Windows 专属行为；
	// filepath.ToSlash 在非 Windows 平台不转换反斜杠，断言仅适用于 Windows。
	if runtime.GOOS != "windows" {
		t.Skip("Windows drive path 编码仅适用于 Windows")
	}
	got := officeProfileURL(`C:\Users\冬冬\AppData\Local\Temp\memora preview\profile`)
	want := "file:///C:/Users/%E5%86%AC%E5%86%AC/AppData/Local/Temp/memora%20preview/profile"
	if got != want {
		t.Fatalf("officeProfileURL() = %q, want %q", got, want)
	}
}

func TestOfficeProfileURLNativeAbsolutePath(t *testing.T) {
	path := t.TempDir()
	got := officeProfileURL(path)
	if !strings.HasPrefix(got, "file:///") {
		t.Fatalf("officeProfileURL(%q) = %q, want an absolute file URI", path, got)
	}
	if runtime.GOOS == "windows" && !strings.Contains(got, ":/") {
		t.Fatalf("officeProfileURL(%q) = %q, want a Windows drive path", path, got)
	}
}

func TestLibreOfficeConvertArgsUsesIsolatedSilentProfile(t *testing.T) {
	profileURL := "file:///C:/Temp/profile"
	args := libreOfficeConvertArgs(profileURL, `C:\Temp\out`, `C:\Temp\source.xlsx`)

	for _, required := range []string{
		"--headless",
		"--nologo",
		"--nofirststartwizard",
		"--norestore",
		"--nodefault",
		"--nolockcheck",
		"-env:UserInstallation=" + profileURL,
	} {
		if !slices.Contains(args, required) {
			t.Errorf("conversion args missing %q: %v", required, args)
		}
	}

	wantTail := []string{"--convert-to", "pdf", "--outdir", `C:\Temp\out`, `C:\Temp\source.xlsx`}
	if !slices.Equal(args[len(args)-len(wantTail):], wantTail) {
		t.Fatalf("conversion args tail = %v, want %v", args[len(args)-len(wantTail):], wantTail)
	}
}
