// Generates rsrc_windows_amd64.syso: app icon (ID 1), manifest (DPI aware, asInvoker), version info.
package main

import (
	"image"
	_ "image/png"
	"log"
	"os"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatal("usage: winresgen icon.png out.syso version")
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	img, _, err := image.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	rs := winres.ResourceSet{}
	icon, err := winres.NewIconFromResizedImage(img, []int{256, 128, 64, 48, 32, 24, 16})
	if err != nil {
		log.Fatal(err)
	}
	if err := rs.SetIcon(winres.ID(1), icon); err != nil {
		log.Fatal(err)
	}
	rs.SetManifest(winres.AppManifest{
		Identity:            winres.AssemblyIdentity{Name: "Emberline.Installer"},
		Description:         "Emberline Server Installer",
		DPIAwareness:        winres.DPIPerMonitorV2,
		ExecutionLevel:      winres.AsInvoker,
		UseCommonControlsV6: true,
	})
	vi := version.Info{}
	vi.SetFileVersion(os.Args[3])
	vi.SetProductVersion(os.Args[3])
	vi.Set(0, version.ProductName, "Emberline Server Installer")
	vi.Set(0, version.FileDescription, "Emberline Server Installer")
	vi.Set(0, version.CompanyName, "Emberline")
	vi.Set(0, version.OriginalFilename, "잔불서버설치.exe")
	vi.Set(0, version.InternalName, "EmberlineInstaller")
	vi.Set(0, version.LegalCopyright, "© 2026 Invictus_1")
	rs.SetVersionInfo(vi)
	out, err := os.Create(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	if err := rs.WriteObject(out, winres.ArchAMD64); err != nil {
		log.Fatal(err)
	}
}
