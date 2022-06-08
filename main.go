/*
Go implementation of Kommiku's reader

Copyright 2022 epiccakeking
Copyright (C) 2019-2021 Valéry Febvre

This file is part of Gomikku.

Gomikku is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

Gomikku is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with Gomikku. If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"flag"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path"

	"database/sql"
	"encoding/json"
	"fmt"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB
var dataPath string
var th *material.Theme

func init() {
	th = material.NewTheme(gofont.Collection())
	th.Fg = color.NRGBA{255, 0, 255, 255}
	home, err := os.UserHomeDir()
	if err != nil {
		panic("Could not find home directory")
	}
	flag.StringVar(&dataPath, "datapath", path.Join(home, ".var/app/info.febvre.Komikku/data"), "Path to data folder")
}

func main() {
	flag.Parse()
	var err error
	db, err = sql.Open("sqlite3", path.Join(dataPath, "komikku.db"))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	go func() {
		w := app.NewWindow(app.Title("Gomikku"))
		err := mangaList(w)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
	getMangas()
}

var keyTag = new(bool)

type mangaButton struct {
	*Manga
	*widget.Clickable
	widget.Image
}

func (m *mangaButton) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: 10, Bottom: 10, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		const r = 20
		size := image.Point{X: gtx.Constraints.Max.X, Y: gtx.Constraints.Max.X * 5 / 4}
		gtx.Constraints.Max = size
		defer clip.RRect{Rect: image.Rectangle{Max: size}, SE: r, SW: r, NW: r, NE: r}.Push(gtx.Ops).Pop()
		return material.Clickable(gtx, m.Clickable, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(m.Image.Layout),
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: r, Bottom: r, Left: r, Right: r}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.SW.Layout(gtx, material.H6(th, m.name).Layout)
					})
				}),
			)
		})
	})

}
func newMangaButton(m *Manga) mangaButton {
	return mangaButton{
		Manga:     m,
		Clickable: new(widget.Clickable),
		Image: widget.Image{
			Src:      paint.NewImageOp(readImage(path.Join(dataPath, m.serverId, m.name, "cover.jpg"))),
			Fit:      widget.Cover,
			Position: layout.Center,
		},
	}
}

func mangaList(w *app.Window) error {
	ops := new(op.Ops)
	mangas := getMangas()
	mangaButtons := make([]mangaButton, len(mangas))
	rendered := 0
	go func() {
		for i := range mangas {
			mangaButtons[i] = newMangaButton(&mangas[i])
			rendered = i
			w.Invalidate()
		}
	}()
	lst := layout.List{Axis: layout.Vertical}

	for e := range w.Events() {
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			gtx := layout.NewContext(ops, e)
			flexes := make([]layout.FlexChild, gtx.Constraints.Max.X/gtx.Dp(unit.Dp(400))+1)
			layout.Inset{Top: 10, Bottom: 10, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return lst.Layout(gtx, (rendered-1)/len(flexes)+1, func(gtx layout.Context, index int) layout.Dimensions {
					for offset := range flexes {
						if i := index*len(flexes) + offset; i < rendered {
							flexes[offset] = layout.Flexed(1, mangaButtons[i].Layout)
						} else {
							flexes[offset] = layout.Flexed(1, layout.Spacer{}.Layout)
						}
					}
					return layout.Flex{
						Axis:    layout.Horizontal,
						Spacing: layout.SpaceAround,
					}.Layout(gtx,
						flexes...,
					)
				})
			})
			e.Frame(gtx.Ops)
			for i := 0; i < rendered; i++ {
				if mangaButtons[i].Clicked() {
					mangaPage(w, &mangas[i])
				}
			}
		}
	}
	return nil
}
func mangaPage(w *app.Window, m *Manga) error {
	w.Invalidate()
	ops := new(op.Ops)
	chapters := getChapters(m)
	lst := layout.List{Axis: layout.Vertical}
	buttons := make([]widget.Clickable, len(chapters))
	for e := range w.Events() {
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			for _, ev := range e.Queue.Events(keyTag) {
				if ev, ok := ev.(key.Event); ok {
					if ev.State == key.Press {
						return nil
					}
				}
			}
			gtx := layout.NewContext(ops, e)
			key.InputOp{
				Tag:  keyTag,
				Hint: key.HintAny,
				Keys: key.NameDeleteBackward,
			}.Add(gtx.Ops)
			key.FocusOp{Tag: keyTag}.Add(ops)
			lst.Layout(gtx, len(chapters), func(gtx layout.Context, index int) layout.Dimensions {
				if chapters[index].Downloaded {
					return material.Button(th, &buttons[index], chapters[index].Title).Layout(gtx)
				}
				return material.Button(th, &buttons[index], chapters[index].Title+" (not downloaded)").Layout(gtx)
			})

			e.Frame(gtx.Ops)
			for i := range buttons {
				if buttons[i].Clicked() {
					if (chapters[i]).Downloaded {
						reader(w, m, chapters[i])
					}
				}
			}
		}
	}
	return nil
}

func reader(w *app.Window, m *Manga, c *Chapter) error {
	w.Invalidate()
	chapterImagePath := path.Join(dataPath, m.serverId, m.name, c.Slug)
	pages := make([]*widget.Image, len(c.Pages))
	loaded := 0
	ops := new(op.Ops)
	lst := layout.List{Axis: layout.Vertical}
	go func() {
		for i := range pages {
			if m.serverId == "webtoon" {
				pages[i] = &widget.Image{
					Src: paint.NewImageOp(readImage(path.Join(chapterImagePath, fmt.Sprintf("%03d.jpeg", i+1)))),
					Fit: widget.Contain,
				}
			} else {
				pages[i] = &widget.Image{
					Src: paint.NewImageOp(readImage(path.Join(chapterImagePath, c.Pages[i].Image))),
					Fit: widget.Contain,
				}
			}
			loaded = i
			if !lst.Position.BeforeEnd {
				w.Invalidate()
			}
		}
	}()
	for e := range w.Events() {
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			for _, ev := range e.Queue.Events(keyTag) {
				if ev, ok := ev.(key.Event); ok {
					if ev.State == key.Press {
						return nil
					}
				}
			}
			gtx := layout.NewContext(ops, e)
			key.InputOp{
				Tag:  keyTag,
				Hint: key.HintAny,
				Keys: key.NameDeleteBackward,
			}.Add(gtx.Ops)
			key.FocusOp{Tag: keyTag}.Add(ops)
			lst.Layout(gtx, loaded, func(gtx layout.Context, index int) layout.Dimensions {
				return pages[index].Layout(gtx)
			})
			e.Frame(gtx.Ops)
		}
	}
	return nil
}

type Chapter struct {
	Title      string
	Slug       string
	Pages      []Chapterimage
	Downloaded bool
}

type Chapterimage struct {
	Image string
	Read  int
	Slug  string
}

func getChapters(m *Manga) []*Chapter {
	q, err := db.Query("select title,slug,pages,downloaded from chapters where manga_id=$1", m.id)
	if err != nil {
		panic(err)
	}
	defer q.Close()
	chapters := []*Chapter{}
	for q.Next() {
		c := new(Chapter)
		var imagesJson string
		q.Scan(&c.Title, &c.Slug, &imagesJson, &c.Downloaded)
		json.Unmarshal([]byte(imagesJson), &c.Pages)
		chapters = append(chapters, c)
	}
	return chapters
}

type Manga struct {
	id       int64
	name     string
	serverId string
}

func getMangas() []Manga {
	q, err := db.Query("select id,name,server_id from mangas order by last_update desc")
	if err != nil {
		panic(err)
	}
	defer q.Close()
	mangas := []Manga{}
	for q.Next() {
		m := Manga{}
		q.Scan(&m.id, &m.name, &m.serverId)
		mangas = append(mangas, m)
	}
	return mangas
}

func readImage(p string) image.Image {
	f, err := os.Open(p)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	i, _, err := image.Decode(f)
	if err != nil {
		panic(err)
	}
	return i
}
