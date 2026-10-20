//go:build !windows

package main

// Preview renderer (non-Windows): draws 10 s strips for many configs into PNGs
// so the waveform engine can be checked without the GUI.

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
)

func main() {
	type c struct {
		name string
		cfg  Config
	}
	var list []c
	for r := Rhythm(0); r < RCount; r++ {
		list = append(list, c{Rhythms[r].Name, Config{Rhythm: r, Rate: Rhythms[r].DefRate}})
	}
	list = append(list,
		c{"Sinus 80 + unifocal PVC bigeminy", Config{Rhythm: RSinus, Rate: 80, Ectopy: EctPVCUni, Pattern: PatBigeminy}},
		c{"Sinus 75 + multifocal PVC occasional 12/min", Config{Rhythm: RSinus, Rate: 75, Ectopy: EctPVCMulti, Pattern: PatOccasional, Freq: 12}},
		c{"Sinus 70 + PVC couplets 6/min", Config{Rhythm: RSinus, Rate: 70, Ectopy: EctPVCUni, Pattern: PatCouplets, Freq: 6}},
		c{"Sinus 70 + PVC runs 6/min", Config{Rhythm: RSinus, Rate: 70, Ectopy: EctPVCUni, Pattern: PatRuns, Freq: 6}},
		c{"Sinus 72 + PAC trigeminy", Config{Rhythm: RSinus, Rate: 72, Ectopy: EctPAC, Pattern: PatTrigeminy}},
		c{"Sinus 72 + PJC quadrigeminy", Config{Rhythm: RSinus, Rate: 72, Ectopy: EctPJC, Pattern: PatQuadrigeminy}},
		c{"AF 100 + PVC 10/min", Config{Rhythm: RAfib, Rate: 100, Ectopy: EctPVCUni, Freq: 10}},
		c{"Sinus 45", Config{Rhythm: RSinus, Rate: 45}},
		c{"Sinus 130", Config{Rhythm: RSinus, Rate: 130}},
		c{"Flutter 150", Config{Rhythm: RAflutter, Rate: 150}},
		c{"Flutter 300 @ 2:1", Config{Rhythm: RAflutter, Rate: 300, FlRatio: Fl2to1}},
		c{"Flutter 300 @ 3:1", Config{Rhythm: RAflutter, Rate: 300, FlRatio: Fl3to1}},
		c{"Flutter 300 @ 4:1 + PVC 8/min", Config{Rhythm: RAflutter, Rate: 300, FlRatio: Fl4to1, Ectopy: EctPVCUni, Freq: 8}},
		c{"Flutter 290 variable", Config{Rhythm: RAflutter, Rate: 290, FlRatio: FlVariable}},
		c{"Paced 70 + PVC bigeminy, SpO2 95", Config{Rhythm: RPaced, Rate: 70, Ectopy: EctPVCUni, Pattern: PatBigeminy, SpO2On: SpO2Monitored, SpO2: 95}},
		c{"Sinus 80 SpO2 84", Config{Rhythm: RSinus, Rate: 80, SpO2On: SpO2Monitored, SpO2: 84}},
		c{"VF SpO2 on", Config{Rhythm: RVFibCoarse, SpO2On: SpO2Monitored, SpO2: 97}},
		c{"AAI fixed 75", Config{Rhythm: RAPaced, Rate: 75}},
		c{"DDD fixed 70", Config{Rhythm: RAVPaced, Rate: 70}},
		c{"VVI demand 60, intrinsic 58", Config{Rhythm: RPaced, Rate: 60, PaceMode: PaceDemand, IntrRate: 58}},
		c{"AAI demand 60, intrinsic 58", Config{Rhythm: RAPaced, Rate: 60, PaceMode: PaceDemand, IntrRate: 58}},
		c{"DDD demand 60, intrinsic 62", Config{Rhythm: RAVPaced, Rate: 60, PaceMode: PaceDemand, IntrRate: 62}},
		c{"Leads off, SpO2 88", Config{Rhythm: RLeadsOff, SpO2On: SpO2Monitored, SpO2: 88}},
	)
	r := rand.New(rand.NewSource(1))
	counts := map[string]int{}
	for i := 0; i < 2000; i++ {
		counts[Rhythms[ERConfig(r, "").Rhythm].Name]++
		RandomConfig(r, "")
	}
	fmt.Println("ER mix over 2000 draws:", counts)
	os.RemoveAll("preview")
	os.MkdirAll("preview", 0755)
	for i, it := range list {
		p := NewPatient(Config{Rhythm: RAsystole}, 100, int64(i+1))
		p.Apply(it.cfg, 100)
		for t := 100.0; t <= 145; t += 0.03 {
			p.Update(t)
		}
		data := p.Snapshot(10)
		var pl []float32
		last := p.Last()
		if it.cfg.SpO2On == SpO2Monitored {
			pl = make([]float32, len(data))
			for j := range pl {
				pl[j] = p.PlethAt(last - int64(len(pl)) + 1 + int64(j))
			}
		}
		t1x := p.TimeOf(last)
		var paces []float64
		for _, tp := range p.PacesBetween(t1x-10, t1x) {
			paces = append(paces, tp-(t1x-10))
		}
		t1 := p.TimeOf(p.Last())
		fmt.Printf("   paces in strip: %d  SpO2=%d valid=%v\n", len(p.PacesBetween(t1-10, t1)), p.SpO2Disp, p.SpO2Valid)
		f, _ := os.Create(fmt.Sprintf("preview/%02d.png", i))
		png.Encode(f, render(data, pl, paces))
		f.Close()
		fmt.Printf("%02d %-50s HR=%d valid=%v alarm=%q\n", i, it.name, p.HR, p.HRValid, p.AlarmText)
	}
}

func render(d, pl []float32, paces []float64) image.Image {
	const pxmm = 4
	w, h := 250*pxmm, 40*pxmm
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 255, 255, 255}
			if x%(5*pxmm) == 0 || y%(5*pxmm) == 0 {
				c = color.RGBA{235, 140, 140, 255}
			} else if x%pxmm == 0 || y%pxmm == 0 {
				c = color.RGBA{250, 210, 210, 255}
			}
			img.Set(x, y, c)
		}
	}
	for _, tp := range paces {
		x := int(tp * float64(w) / 10)
		for y := 0; y < h/2; y++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255})
		}
	}
	for i, v := range pl {
		x := i * w / len(pl)
		img.Set(x, int(float64(h)*0.95-float64(v)*30), color.RGBA{0, 120, 255, 255})
		img.Set(x, int(float64(h)*0.95-float64(v)*30)+1, color.RGBA{0, 120, 255, 255})
	}
	base := float64(h) * 0.55
	py := -1
	for i, v := range d {
		x := i * w / len(d)
		y := int(base - float64(v)*10*pxmm)
		if y < 0 {
			y = 0
		}
		if y >= h {
			y = h - 1
		}
		if py < 0 {
			py = y
		}
		a, b := py, y
		if a > b {
			a, b = b, a
		}
		for yy := a; yy <= b; yy++ {
			img.Set(x, yy, color.Black)
			img.Set(x+1, yy, color.Black)
		}
		py = y
	}
	return img
}
