// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"goki.dev/colors"
	"goki.dev/gi/v2/gi"
	"goki.dev/gi/v2/gimain"
	"goki.dev/gi/v2/giv"
	"goki.dev/girl/styles"
	"goki.dev/girl/units"
	"goki.dev/icons"
	"goki.dev/ki/v2"
	"goki.dev/mat32/v2"
)

// FileBrowse is a simple file browser / viewer / editor with a file tree and
// one or more editor windows.  It is based on an early version of the Gide
// IDE framework, and remains simple to test / demo the file tree component.
type FileBrowse struct {
	gi.Frame

	// root directory for the project -- all projects must be organized within a top-level root directory, with all the files therein constituting the scope of the project -- by default it is the path for ProjFilename
	ProjRoot gi.FileName `desc:"root directory for the project -- all projects must be organized within a top-level root directory, with all the files therein constituting the scope of the project -- by default it is the path for ProjFilename"`

	// filename of the currently-active textview
	ActiveFilename gi.FileName `desc:"filename of the currently-active textview"`

	// has the root changed?  we receive update signals from root for changes
	Changed bool `json:"-" desc:"has the root changed?  we receive update signals from root for changes"`

	// all the files in the project directory and subdirectories
	Files giv.FileTree `desc:"all the files in the project directory and subdirectories"`

	// treeview of all the files in the project directory and subdirectories
	FilesView *giv.FileTreeView `desc:"treeview of all the files in the project directory and subdirectories"`

	// number of textviews available for editing files (default 2) -- configurable with n-text-views property
	NTextViews int `xml:"n-text-views" desc:"number of textviews available for editing files (default 2) -- configurable with n-text-views property"`

	// index of the currently-active textview -- new files will be viewed in other views if available
	ActiveTextViewIdx int `json:"-" desc:"index of the currently-active textview -- new files will be viewed in other views if available"`
}

func (fb *FileBrowse) OnInit() {
	fb.AddStyles(func(s *styles.Style) {
		s.BackgroundColor.SetSolid(colors.Scheme.Background)
		s.Color = colors.Scheme.OnBackground
		s.SetStretchMax()
		s.Margin.Set(units.Dp(8 ))
	})
}

func (fb *FileBrowse) OnChildAdded(child ki.Ki) {
	w, _ := gi.AsWidget(child)
	switch w.Name() {
	case "title":
		title := w.(*gi.Label)
		title.Type = gi.LabelHeadlineSmall
		w.AddStyles(func(s *styles.Style) {
			s.SetStretchMaxWidth()
			s.AlignH = styles.AlignCenter
			s.AlignV = styles.AlignTop
		})
	case "splitview":
		split := w.(*gi.Splits)
		split.Dim = mat32.X
	}
	ip, _ := w.IndexInParent()
	if w.Parent().Name() == "splitview" && ip > 0 {
		w.AddStyles(func(s *styles.Style) {
			s.SetStretchMax()
			s.SetMinPrefWidth(units.Ch(20))
			s.SetMinPrefHeight(units.Ch(10))
			s.Font.Family = string(gi.Prefs.MonoFont)
			s.Text.WhiteSpace = styles.WhiteSpacePreWrap
			s.Text.TabSize = 4
		})
	}
}

// UpdateFiles updates the list of files saved in project
func (fb *FileBrowse) UpdateFiles() {
	if fb.FilesView == nil {
		fb.Files.OpenPath(string(fb.ProjRoot))
	} else {
		updt := fb.FilesView.UpdateStart()
		fb.FilesView.SetFullReRender()
		fb.Files.OpenPath(string(fb.ProjRoot))
		fb.FilesView.UpdateEnd(updt)
	}
}

// IsEmpty returns true if given FileBrowse project is empty -- has not been set to a valid path
func (fb *FileBrowse) IsEmpty() bool {
	return fb.ProjRoot == ""
}

// OpenPath opens a new browser viewer at given path, which can either be a
// specific file or a directory containing multiple files of interest -- opens
// in current FileBrowse object if it is empty, or otherwise opens a new
// window.
func (fb *FileBrowse) OpenPath(path gi.FileName) {
	if !fb.IsEmpty() {
		NewFileBrowser(string(path))
		return
	}
	fb.Defaults()
	root, pnm, fnm, ok := ProjPathParse(string(path))
	if ok {
		fb.ProjRoot = gi.FileName(root)
		fb.SetName(pnm)
		fb.UpdateProj()
		win := fb.ParentRenderWin()
		if win != nil {
			winm := "browser-" + pnm
			win.SetName(winm)
			win.SetTitle(winm)
		}
		if fnm != "" {
			fb.ViewFile(fnm)
		}
		fb.UpdateFiles()
	}
}

// UpdateProj does full update to current proj
func (fb *FileBrowse) UpdateProj() {
	mods, updt := fb.StdConfig()
	fb.SetTitle(fmt.Sprintf("FileBrowse of: %v", fb.ProjRoot)) // todo: get rid of title
	fb.UpdateFiles()
	fb.ConfigSplitView()
	fb.ConfigToolbar()
	if mods {
		fb.UpdateEnd(updt)
	}
}

// ProjPathParse parses given project path into a root directory (which could
// be the path or just the directory portion of the path, depending in whether
// the path is a directory or not), and a bool if all is good (otherwise error
// message has been reported). projnm is always the last directory of the path.
func ProjPathParse(path string) (root, projnm, fnm string, ok bool) {
	if path == "" {
		return "", "blank", "", false
	}
	info, err := os.Lstat(path)
	if err != nil {
		emsg := fmt.Errorf("ProjPathParse: Cannot open at given path: %q: Error: %v", path, err)
		log.Println(emsg)
		return
	}
	dir, fn := filepath.Split(path)
	pathIsDir := info.IsDir()
	if pathIsDir {
		root = path
	} else {
		root = dir
		fnm = fn
	}
	_, projnm = filepath.Split(root)
	ok = true
	return
}

//////////////////////////////////////////////////////////////////////////////////////
//   TextViews

// ActiveTextView returns the currently-active TextView
func (fb *FileBrowse) ActiveTextView() *giv.TextView {
	return fb.TextViewByIndex(fb.ActiveTextViewIdx)
}

// SetActiveTextView sets the given view index as the currently-active
// TextView -- returns that textview
func (fb *FileBrowse) SetActiveTextView(idx int) *giv.TextView {
	if idx < 0 || idx >= fb.NTextViews {
		log.Printf("FileBrowse SetActiveTextView: text view index out of range: %v\n", idx)
		return nil
	}
	fb.ActiveTextViewIdx = idx
	av := fb.ActiveTextView()
	if av.Buf != nil {
		fb.ActiveFilename = av.Buf.Filename
	}
	av.GrabFocus()
	return av
}

// NextTextView returns the next text view available for viewing a file and
// its index -- if the active text view is empty, then it is used, otherwise
// it is the next one
func (fb *FileBrowse) NextTextView() (*giv.TextView, int) {
	av := fb.TextViewByIndex(fb.ActiveTextViewIdx)
	if av.Buf == nil {
		return av, fb.ActiveTextViewIdx
	}
	nxt := (fb.ActiveTextViewIdx + 1) % fb.NTextViews
	return fb.TextViewByIndex(nxt), nxt
}

// SaveActiveView saves the contents of the currently-active textview
func (fb *FileBrowse) SaveActiveView() {
	tv := fb.ActiveTextView()
	if tv.Buf != nil {
		tv.Buf.Save() // todo: errs..
		fb.UpdateFiles()
	}
}

// SaveActiveViewAs save with specified filename the contents of the
// currently-active textview
func (fb *FileBrowse) SaveActiveViewAs(filename gi.FileName) {
	tv := fb.ActiveTextView()
	if tv.Buf != nil {
		tv.Buf.SaveAs(filename)
	}
}

// ViewFileNode sets the next text view to view file in given node (opens
// buffer if not already opened)
func (fb *FileBrowse) ViewFileNode(fn *giv.FileNode) {
	if _, err := fn.OpenBuf(); err == nil {
		nv, nidx := fb.NextTextView()
		if nv.Buf != nil && nv.Buf.IsChanged() { // todo: save current changes?
			fmt.Printf("Changes not saved in file: %v before switching view there to new file\n", nv.Buf.Filename)
		}
		nv.SetBuf(fn.Buf)
		fn.Buf.Hi.Style = "emacs" // todo prefs
		fb.SetActiveTextView(nidx)
		fb.UpdateFiles()
	}
}

// ViewFile sets the next text view to view given file name -- include as much
// of name as possible to disambiguate -- will use the first matching --
// returns false if not found
func (fb *FileBrowse) ViewFile(fnm string) bool {
	fn, ok := fb.Files.FindFile(fnm)
	if !ok {
		return false
	}
	fb.ViewFileNode(fn)
	return true
}

//////////////////////////////////////////////////////////////////////////////////////
//    Defaults, Prefs

func (fb *FileBrowse) Defaults() {
	fb.NTextViews = 2
	fb.Files.DirsOnTop = true
}

//////////////////////////////////////////////////////////////////////////////////////
//   GUI configs

// StdFrameConfig returns a Config for configuring a standard Frame
// -- can modify as desired before calling ConfigChildren on Frame using this
func (fb *FileBrowse) StdFrameConfig() ki.Config {
	config := ki.Config{}
	config.Add(gi.LabelType, "title")
	config.Add(gi.ToolBarType, "toolbar")
	config.Add(gi.TypeSplitView, "splitview")
	return config
}

// StdConfig configures a standard setup of the overall Frame -- returns mods,
// updt from ConfigChildren and does NOT call UpdateEnd
func (fb *FileBrowse) StdConfig() (mods, updt bool) {
	fb.Lay = gi.LayoutVert
	fb.SetProp("spacing", gi.StdDialogVSpaceUnits)
	config := fb.StdFrameConfig()
	mods, updt = fb.ConfigChildren(config)
	return
}

// SetTitle sets the optional title and updates the Title label
func (fb *FileBrowse) SetTitle(title string) {
	lab, _ := fb.TitleWidget()
	if lab != nil {
		lab.Text = title
	}
}

// Title returns the title label widget, and its index, within frame -- nil,
// -1 if not found
func (fb *FileBrowse) TitleWidget() (*gi.Label, int) {
	idx, ok := fb.Children().IndexByName("title", 0)
	if !ok {
		return nil, -1
	}
	return fb.Child(idx).(*gi.Label), idx
}

// SplitView returns the main SplitView
func (fb *FileBrowse) SplitView() (*gi.Splits, int) {
	idx, ok := fb.Children().IndexByName("splitview", 2)
	if !ok {
		return nil, -1
	}
	return fb.Child(idx).(*gi.Splits), idx
}

// TextViewByIndex returns the TextView by index, nil if not found
func (fb *FileBrowse) TextViewByIndex(idx int) *giv.TextView {
	if idx < 0 || idx >= fb.NTextViews {
		log.Printf("FileBrowse: text view index out of range: %v\n", idx)
		return nil
	}
	split, _ := fb.SplitView()
	stidx := 1 // 0 = file browser -- could be collapsed but always there.
	if split != nil {
		svk := split.Child(stidx + idx).Child(0)
		if !ki.TypeEmbeds(svk, giv.TypeTextView) {
			log.Printf("FileBrowse: text view not at index: %v\n", idx)
			return nil
		}
		return svk.(*giv.TextView)
	}
	return nil
}

// ToolBar returns the toolbar widget
func (fb *FileBrowse) ToolBar() *gi.ToolBar {
	idx, ok := fb.Children().IndexByName("toolbar", 1)
	if !ok {
		return nil
	}
	return fb.Child(idx).(*gi.ToolBar)
}

// ConfigToolbar adds a FileBrowse toolbar.
func (fb *FileBrowse) ConfigToolbar() {
	tb := fb.ToolBar()
	if tb.HasChildren() {
		return
	}
	tb.SetStretchMaxWidth()
	giv.ToolBarView(fb, fb.Scene, tb)
}

// SplitViewConfig returns a Config for configuring the SplitView
func (fb *FileBrowse) SplitViewConfig() ki.Config {
	config := ki.Config{}
	config.Add(gi.FrameType, "filetree-fr")
	for i := 0; i < fb.NTextViews; i++ {
		config.Add(gi.LayoutType, fmt.Sprintf("textview-lay-%v", i))
	}
	// todo: tab view
	return config
}

// ConfigSplitView configures the SplitView.
func (fb *FileBrowse) ConfigSplitView() {
	split, _ := fb.SplitView()
	if split == nil {
		return
	}

	config := fb.SplitViewConfig()
	mods, updt := split.ConfigChildren(config)
	if mods {
		ftfr := split.Child(0).(*gi.Frame)
		ft := giv.NewFileTreeView(ftfr, "filetree")
		fb.FilesView = ft
		ft.SetRootNode(&fb.Files)

		for i := 0; i < fb.NTextViews; i++ {
			txly := split.Child(1 + i).(*gi.Layout)

			txed := giv.NewTextView(txly, "textview-"+strconv.Itoa(i))
			txed.Scene = fb.Scene
		}

		ft.TreeViewSig.Connect(fb.This(), func(recv, send ki.Ki, sig int64, data any) {
			if data == nil {
				return
			}
			tvn, _ := data.(ki.Ki).Embed(giv.TypeFileTreeView).(*giv.FileTreeView)
			fbb, _ := recv.Embed(TypeFileBrowse).(*FileBrowse)
			fn := tvn.SrcNode.Embed(giv.FileNodeType).(*giv.FileNode)
			switch sig {
			case int64(giv.TreeViewSelected):
				fbb.FileNodeSelected(fn, tvn)
			case int64(giv.TreeViewOpened):
				fbb.FileNodeOpened(fn, tvn)
			case int64(giv.TreeViewClosed):
				fbb.FileNodeClosed(fn, tvn)
			}
		})
		split.SetSplits(.2, .4, .4)
		split.UpdateEnd(updt)
	}
}

func (fb *FileBrowse) FileNodeSelected(fn *giv.FileNode, tvn *giv.FileTreeView) {
}

func (fb *FileBrowse) FileNodeOpened(fn *giv.FileNode, tvn *giv.FileTreeView) {
	if fn.IsDir() {
		if !fn.IsOpen() {
			tvn.SetOpen()
			fn.OpenDir()
		}
	} else {
		fb.ViewFileNode(fn)
		fn.SetOpen()
		fn.UpdateNode()
	}
}

func (fb *FileBrowse) FileNodeClosed(fn *giv.FileNode, tvn *giv.FileTreeView) {
	if fn.IsDir() {
		if fn.IsOpen() {
			fn.CloseDir()
		}
	}
}

func (fb *FileBrowse) Render(vp *Scene) {
	fb.ToolBar().UpdateButtons()
	if win := fb.ParentRenderWin(); win != nil {
		if !win.Is(WinResizing) {
			win.MainMenuUpdateActives()
		}
	}
	fb.Frame.Render()
}

var FileBrowseProps = ki.Props{
	"ToolBar": ki.PropSlice{
		{"UpdateFiles", ki.Props{
			"shortcut": "Command+U",
			"icon":     icons.Refresh,
		}},
		{"SaveActiveView", ki.Props{
			"label": "Save",
			"icon":  icons.Save,
		}},
		{"SaveActiveViewAs", ki.Props{
			"label": "Save As...",
			"icon":  icons.SaveAs,
			"Args": ki.PropSlice{
				{"File Name", ki.Props{
					"default-field": "ActiveFilename",
				}},
			},
		}},
	},
	"MainMenu": ki.PropSlice{
		{"AppMenu", ki.BlankProp{}},
		{"File", ki.PropSlice{
			{"OpenPath", ki.Props{
				"shortcut":        gi.KeyFunMenuOpen,
				"no-update-after": true,
				"Args": ki.PropSlice{
					{"Path", ki.Props{
						"dirs-only": true, // todo: support
					}},
				},
			}},
			{"sep-close", ki.BlankProp{}},
			{"Close RenderWin", ki.BlankProp{}},
		}},
		{"Edit", "Copy Cut Paste"},
		{"RenderWin", "RenderWins"},
	},
}

//////////////////////////////////////////////////////////////////////////////////////
//   Project window

// NewFileBrowser creates a new FileBrowse window with a new FileBrowse project for given
// path, returning the window and the path
func NewFileBrowser(path string) (*gi.RenderWin, *FileBrowse) {
	_, projnm, _, _ := ProjPathParse(path)
	winm := "browser-" + projnm

	width := 1280
	height := 720

	win := gi.NewMainRenderWin(winm, winm, width, height)

	vp := win.WinScene()
	updt := vp.UpdateStart()

	mfr := win.SetMainFrame()

	fb := NewFileBrowse(mfr, "browser")
	fb.Scene = vp

	fb.OpenPath(gi.FileName(path))

	mmen := win.MainMenu
	giv.MainMenuView(fb, win, mmen)

	inClosePrompt := false
	win.SetCloseReqFunc(func(w *gi.RenderWin) {
		if !inClosePrompt {
			inClosePrompt = true
			// if fb.Changed {
			gi.ChoiceDialog(vp, gi.DlgOpts{Title: "Close Without Saving?",
				Prompt: "Do you want to save your changes?  If so, Cancel and then Save"},
				[]string{"Close Without Saving", "Cancel"},
				win.This(), func(recv, send ki.Ki, sig int64, data any) {
					switch sig {
					case 0:
						w.Close()
					case 1:
						// default is to do nothing, i.e., cancel
					}
				})
			// } else {
			// 	w.Close()
			// }
		}
	})

	inQuitPrompt := false
	gi.SetQuitReqFunc(func() {
		if !inQuitPrompt {
			inQuitPrompt = true
			gi.PromptDialog(vp, gi.DlgOpts{Title: "Really Quit?",
				Prompt: "Are you <i>sure</i> you want to quit?", Ok: true, Cancel: true}, fun func(dlg *gi.DialogStage)) *gi.DialogStage {
					if dlg.Accepted {
						gi.Quit()
					} else {
						inQuitPrompt = false
					}
				})
		}
	})

	// win.SetCloseCleanFunc(func(w *gi.RenderWin) {
	// 	fmt.Printf("Doing final Close cleanup here..\n")
	// })

	win.SetCloseCleanFunc(func(w *gi.RenderWin) {
		if gi.MainRenderWins.Len() <= 1 {
			go gi.Quit() // once main window is closed, quit
		}
	})

	win.MainMenuUpdated()

	vp.UpdateEndNoSig(updt)

	win.GoStartEventLoop()
	return win, fb
}

//////////////////////////////////////////////////////////////////////////////////////
//  main

func main() { gimain.Run(app) }

func app() {
	gi.SetAppName("file-browser")
	gi.SetAppAbout(`<code>FileBrowser</code> is a demo / test of the FileTree / FileNode browser in the <b>GoGi</b> graphical interface system, within the <b>GoKi</b> tree framework.  See <a href="https://goki.dev/gi/v2de">gide on GitHub</a>`)

	// gi.SetQuitCleanFunc(func() {
	// 	fmt.Printf("Doing final Quit cleanup here..\n")
	// })

	var path string

	// process command args
	if len(os.Args) > 1 {
		flag.StringVar(&path, "path", "./", "path to open -- can be to a directory or a filename within the directory")
		// todo: other args?
		flag.Parse()
		if path == "" {
			if flag.NArg() > 0 {
				path = flag.Arg(0)
			}
		}
	}

	if path != "" {
		path, _ = filepath.Abs(path)
	}
	NewFileBrowser(path)
	// above calls will have added to WinWait..
	gi.WinWait.Wait()
}
