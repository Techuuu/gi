// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package giv

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/vcs"
	"github.com/fsnotify/fsnotify"
	"goki.dev/colors"
	"goki.dev/gi/v2/gi"
	"goki.dev/gi/v2/giv/textbuf"
	"goki.dev/gi/v2/histyle"
	"goki.dev/girl/states"
	"goki.dev/girl/styles"
	"goki.dev/goosi"
	"goki.dev/goosi/events"
	"goki.dev/goosi/mimedata"
	"goki.dev/grr"
	"goki.dev/gti"
	"goki.dev/icons"
	"goki.dev/ki/v2"
	"goki.dev/pi/v2/filecat"
	"goki.dev/vci/v2"
)

// DirAndFile returns the final dir and file name.
func DirAndFile(file string) string {
	dir, fnm := filepath.Split(file)
	return filepath.Join(filepath.Base(dir), fnm)
}

// RelFilePath returns the file name relative to given root file path, if it is
// under that root -- otherwise it returns the final dir and file name.
func RelFilePath(file, root string) string {
	rp, err := filepath.Rel(root, file)
	if err == nil && !strings.HasPrefix(rp, "..") {
		return rp
	}
	return DirAndFile(file)
}

const (
	// FileTreeExtFilesName is the name of the node that represents external files
	FileTreeExtFilesName = "[external files]"
)

// FileTree is the root of a tree representing files in a given directory (and
// subdirectories thereof), and has some overall management state for how to
// view things.  The FileTree can be viewed by a TreeView to provide a GUI
// interface into it.
type FileTree struct {
	FileNode

	// external files outside the root path of the tree -- abs paths are stored -- these are shown in the first sub-node if present -- use AddExtFile to add and update
	ExtFiles []string `desc:"external files outside the root path of the tree -- abs paths are stored -- these are shown in the first sub-node if present -- use AddExtFile to add and update"`

	// records state of directories within the tree (encoded using paths relative to root), e.g., open (have been opened by the user) -- can persist this to restore prior view of a tree
	Dirs DirFlagMap `desc:"records state of directories within the tree (encoded using paths relative to root), e.g., open (have been opened by the user) -- can persist this to restore prior view of a tree"`

	// if true, then all directories are placed at the top of the tree view -- otherwise everything is mixed
	DirsOnTop bool `desc:"if true, then all directories are placed at the top of the tree view -- otherwise everything is mixed"`

	// [view: -] type of node to create -- defaults to giv.FileNode but can use custom node types
	NodeType *gti.Type `view:"-" json:"-" xml:"-" desc:"type of node to create -- defaults to giv.FileNode but can use custom node types"`

	// if true, we are in midst of an OpenAll call -- nodes should open all dirs
	InOpenAll bool `desc:"if true, we are in midst of an OpenAll call -- nodes should open all dirs"`

	// [view: -] change notify for all dirs
	Watcher *fsnotify.Watcher `view:"-" desc:"change notify for all dirs"`

	// [view: -] channel to close watcher watcher
	DoneWatcher chan bool `view:"-" desc:"channel to close watcher watcher"`

	// [view: -] map of paths that have been added to watcher -- only active if bool = true
	WatchedPaths map[string]bool `view:"-" desc:"map of paths that have been added to watcher -- only active if bool = true"`

	// [view: -] last path updated by watcher
	LastWatchUpdt string `view:"-" desc:"last path updated by watcher"`

	// [view: -] timestamp of last update
	LastWatchTime time.Time `view:"-" desc:"timestamp of last update"`

	// [view: -] Update mutex
	UpdtMu sync.Mutex `view:"-" desc:"Update mutex"`
}

func (ft *FileTree) CopyFieldsFrom(frm any) {
	fr := frm.(*FileTree)
	ft.FileNode.CopyFieldsFrom(&fr.FileNode)
	ft.DirsOnTop = fr.DirsOnTop
	ft.NodeType = fr.NodeType
}

func (fv *FileTree) Disconnect() {
	if fv.Watcher != nil {
		fv.Watcher.Close()
		fv.Watcher = nil
	}
	if fv.DoneWatcher != nil {
		fv.DoneWatcher <- true
		close(fv.DoneWatcher)
		fv.DoneWatcher = nil
	}
	// todo: no Disconnect anymore!
	// fv.FileNode.Disconnect()
}

// OpenPath opens a filetree at given directory path -- reads all the files at
// given path into this tree -- uses config children to preserve extra info
// already stored about files.  Only paths listed in Dirs will be opened.
func (ft *FileTree) OpenPath(path string) {
	ft.FRoot = ft // we are our own root..
	if ft.NodeType == nil {
		ft.NodeType = FileNodeType
	}
	effpath, err := filepath.EvalSymlinks(path)
	if err != nil {
		effpath = path
	}
	abs, err := filepath.Abs(effpath)
	if err != nil {
		log.Printf("giv.FileTree:OpenPath: %s\n", err)
		abs = effpath
	}
	ft.FPath = gi.FileName(abs)
	ft.UpdateAll()
}

// UpdateAll does a full update of the tree -- calls ReadDir on current path
func (ft *FileTree) UpdateAll() {
	ft.UpdtMu.Lock()
	ft.Dirs.ClearMarks()
	ft.ReadDir(string(ft.FPath))
	// the problem here is that closed dirs are not visited but we want to keep their settings:
	// ft.Dirs.DeleteStale()
	ft.UpdtMu.Unlock()
}

// UpdatePath updates the tree at the directory level for given path
// and everything below it
func (ft *FileTree) UpdatePath(path string) {
	ft.UpdtMu.Lock()
	ft.UpdtMu.Unlock()
}

// todo: rewrite below to use UpdatePath

// UpdateNewFile should be called with path to a new file that has just been
// created -- will update view to show that file, and if that file doesn't
// exist, it updates the directory containing that file
func (ft *FileTree) UpdateNewFile(filename string) {
	ft.DirsTo(filename)
	fpath, _ := filepath.Split(filename)
	fpath = filepath.Clean(fpath)
	if fn, ok := ft.FindFile(filename); ok {
		// fmt.Printf("updating node for file: %v\n", filename)
		fn.UpdateNode()
	} else if fn, ok := ft.FindFile(fpath); ok {
		// fmt.Printf("updating node for path: %v\n", fpath)
		fn.UpdateNode()
		// } else {
		// log.Printf("giv.FileTree UpdateNewFile: no node found for path to update: %v\n", filename)
	}
}

// ConfigWatcher configures a new watcher for tree
func (ft *FileTree) ConfigWatcher() error {
	if ft.Watcher != nil {
		return nil
	}
	ft.WatchedPaths = make(map[string]bool)
	var err error
	ft.Watcher, err = fsnotify.NewWatcher()
	return err
}

// WatchWatcher monitors the watcher channel for update events.
// It must be called once some paths have been added to watcher --
// safe to call multiple times.
func (ft *FileTree) WatchWatcher() {
	if ft.Watcher == nil || ft.Watcher.Events == nil {
		return
	}
	if ft.DoneWatcher != nil {
		return
	}
	ft.DoneWatcher = make(chan bool)
	go func() {
		watch := ft.Watcher
		done := ft.DoneWatcher
		for {
			select {
			case <-done:
				return
			case event := <-watch.Events:
				switch {
				case event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Remove == fsnotify.Remove ||
					event.Op&fsnotify.Rename == fsnotify.Rename:
					ft.WatchUpdt(event.Name)
				}
			case err := <-watch.Errors:
				_ = err
			}
		}
	}()
}

// WatchUpdt does the update for given path
func (ft *FileTree) WatchUpdt(path string) {
	ft.UpdtMu.Lock()
	defer ft.UpdtMu.Unlock()
	// fmt.Println(path)

	dir, _ := filepath.Split(path)
	rp := ft.RelPath(gi.FileName(dir))
	if rp == ft.LastWatchUpdt {
		now := time.Now()
		lagMs := int(now.Sub(ft.LastWatchTime) / time.Millisecond)
		if lagMs < 100 {
			// fmt.Printf("skipping update to: %s  due to lag: %v\n", rp, lagMs)
			return // no update
		}
	}
	fn, err := ft.FindDirNode(rp)
	if err != nil {
		// log.Println(err)
		return
	}
	ft.LastWatchUpdt = rp
	ft.LastWatchTime = time.Now()
	if !fn.IsOpen() {
		// fmt.Printf("warning: watcher updating closed node: %s\n", rp)
		return // shouldn't happen
	}
	// update node
	fn.UpdateNode()
}

// WatchPath adds given path to those watched
func (ft *FileTree) WatchPath(path gi.FileName) error {
	return nil // disable for all platforms for now -- getting some issues
	if goosi.TheApp.Platform() == goosi.MacOS {
		return nil // mac is not supported in a high-capacity fashion at this point
	}
	rp := ft.RelPath(path)
	on, has := ft.WatchedPaths[rp]
	if on || has {
		return nil
	}
	ft.ConfigWatcher()
	// fmt.Printf("watching path: %s\n", path)
	err := ft.Watcher.Add(string(path))
	if err == nil {
		ft.WatchedPaths[rp] = true
		ft.WatchWatcher()
	} else {
		log.Println(err)
	}
	return err
}

// UnWatchPath removes given path from those watched
func (ft *FileTree) UnWatchPath(path gi.FileName) {
	rp := ft.RelPath(path)
	on, has := ft.WatchedPaths[rp]
	if !on || !has {
		return
	}
	ft.ConfigWatcher()
	ft.Watcher.Remove(string(path))
	ft.WatchedPaths[rp] = false
}

// IsDirOpen returns true if given directory path is open (i.e., has been
// opened in the view)
func (ft *FileTree) IsDirOpen(fpath gi.FileName) bool {
	if fpath == ft.FPath { // we are always open
		return true
	}
	return ft.Dirs.IsOpen(ft.RelPath(fpath))
}

// SetDirOpen sets the given directory path to be open
func (ft *FileTree) SetDirOpen(fpath gi.FileName) {
	rp := ft.RelPath(fpath)
	// fmt.Printf("setdiropen: %s\n", rp)
	ft.Dirs.SetOpen(rp, true)
	ft.Dirs.SetMark(rp)
	ft.WatchPath(fpath)
}

// SetDirClosed sets the given directory path to be closed
func (ft *FileTree) SetDirClosed(fpath gi.FileName) {
	rp := ft.RelPath(fpath)
	ft.Dirs.SetOpen(rp, false)
	ft.Dirs.SetMark(rp)
	ft.UnWatchPath(fpath)
}

// SetDirSortBy sets the given directory path sort by option
func (ft *FileTree) SetDirSortBy(fpath gi.FileName, modTime bool) {
	ft.Dirs.SetSortBy(ft.RelPath(fpath), modTime)
}

// DirSortByName returns true if dir is sorted by name
func (ft *FileTree) DirSortByName(fpath gi.FileName) bool {
	return ft.Dirs.SortByName(ft.RelPath(fpath))
}

// DirSortByModTime returns true if dir is sorted by mod time
func (ft *FileTree) DirSortByModTime(fpath gi.FileName) bool {
	return ft.Dirs.SortByModTime(ft.RelPath(fpath))
}

// AddExtFile adds an external file outside of root of file tree
// and triggers an update, returning the FileNode for it, or
// error if filepath.Abs fails.
func (ft *FileTree) AddExtFile(fpath string) (*FileNode, error) {
	pth, err := filepath.Abs(fpath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(pth); err != nil {
		return nil, err
	}
	if has, _ := ft.HasExtFile(pth); has {
		return ft.ExtFileNodeByPath(pth)
	}
	ft.ExtFiles = append(ft.ExtFiles, pth)
	ft.UpdateDir()
	return ft.ExtFileNodeByPath(pth)
}

// RemoveExtFile removes external file from maintained list,  returns true if removed
func (ft *FileTree) RemoveExtFile(fpath string) bool {
	for i, ef := range ft.ExtFiles {
		if ef == fpath {
			ft.ExtFiles = append(ft.ExtFiles[:i], ft.ExtFiles[i+1:]...)
			return true
		}
	}
	return false
}

// HasExtFile returns true and index if given abs path exists on ExtFiles list.
// false and -1 if not.
func (ft *FileTree) HasExtFile(fpath string) (bool, int) {
	for i, f := range ft.ExtFiles {
		if f == fpath {
			return true, i
		}
	}
	return false, -1
}

// ExtFileNodeByPath returns FileNode for given file path, and true, if it
// exists in the external files list.  Otherwise returns nil, false.
func (ft *FileTree) ExtFileNodeByPath(fpath string) (*FileNode, error) {
	ehas, i := ft.HasExtFile(fpath)
	if !ehas {
		return nil, fmt.Errorf("ExtFile not found on list: %v", fpath)
	}
	ekid, err := ft.ChildByNameTry(FileTreeExtFilesName, 0)
	if err != nil {
		return nil, fmt.Errorf("ExtFile not updated -- no ExtFiles node")
	}
	ekids := *ekid.Children()
	err = ekids.IsValidIndex(i)
	if err == nil {
		kn := AsFileNode(ekids.Elem(i))
		return kn, nil
	}
	return nil, fmt.Errorf("ExtFile not updated: %v", err)
}

// UpdateExtFiles returns a type-and-name list for configuring nodes
// for ExtFiles
func (ft *FileTree) UpdateExtFiles(efn *FileNode) {
	efn.Info.Mode = os.ModeDir | os.ModeIrregular // mark as dir, irregular
	efn.SetOpen()
	config := ki.Config{}
	typ := ft.NodeType
	for _, f := range ft.ExtFiles {
		config.Add(typ, DirAndFile(f))
	}
	mods, updt := efn.ConfigChildren(config) // NOT unique names
	if mods {
		// fmt.Printf("got mods: %v\n", path)
	}
	// always go through kids, regardless of mods
	for i, sfk := range efn.Kids {
		sf := AsFileNode(sfk)
		sf.FRoot = ft
		fp := ft.ExtFiles[i]
		sf.SetNodePath(fp)
		sf.Info.Vcs = vci.Stored // no vcs in general
	}
	if mods {
		efn.UpdateEnd(updt)
	}
}

//////////////////////////////////////////////////////////////////////////////
//    FileNode

// FileNodeHiStyle is the default style for syntax highlighting to use for
// file node buffers
var FileNodeHiStyle = histyle.StyleDefault

// FileNode represents a file in the file system -- the name of the node is
// the name of the file.  Folders have children containing further nodes.
//
//goki:embedder
type FileNode struct {
	ki.Node

	// full path to this file
	FPath gi.FileName `json:"-" xml:"-" copy:"-" desc:"full path to this file"`

	// full standard file info about this file
	Info FileInfo `json:"-" xml:"-" copy:"-" desc:"full standard file info about this file"`

	// file buffer for editing this file
	Buf *TextBuf `json:"-" xml:"-" copy:"-" desc:"file buffer for editing this file"`

	// root of the tree -- has global state
	FRoot *FileTree `json:"-" xml:"-" copy:"-" desc:"root of the tree -- has global state"`

	// version control system repository for this directory, only non-nil if this is the highest-level directory in the tree under vcs control
	DirRepo vci.Repo `json:"-" xml:"-" copy:"-" desc:"version control system repository for this directory, only non-nil if this is the highest-level directory in the tree under vcs control"`

	// version control system repository file status -- only valid during ReadDir
	RepoFiles vci.Files `json:"-" xml:"-" copy:"-" desc:"version control system repository file status -- only valid during ReadDir"`
}

func (fn *FileNode) CopyFieldsFrom(frm any) {
	// note: not copying ki.Node as it doesn't have any copy fields
	// fr := frm.(*FileNode)
	// and indeed nothing here should be copied!
}

// IsDir returns true if file is a directory (folder)
func (fn *FileNode) IsDir() bool {
	return fn.Info.IsDir()
}

// IsIrregular  returns true if file is a special "Irregular" node
func (fn *FileNode) IsIrregular() bool {
	return (fn.Info.Mode & os.ModeIrregular) != 0
}

// IsExternal returns true if file is external to main file tree
func (fn *FileNode) IsExternal() bool {
	isExt := false
	fn.WalkUp(func(k ki.Ki) bool {
		sfn := AsFileNode(k)
		if sfn == nil {
			return ki.Break
		}
		if sfn.IsIrregular() {
			isExt = true
			return ki.Break
		}
		return ki.Continue
	})
	return isExt
}

// HasClosedParent returns true if node has a parent node with !IsOpen flag set
func (fn *FileNode) HasClosedParent() bool {
	hasClosed := false
	fn.WalkUpParent(func(k ki.Ki) bool {
		sfn := AsFileNode(k)
		if sfn == nil {
			return ki.Break
		}
		if !sfn.IsOpen() {
			hasClosed = true
			return ki.Break
		}
		return ki.Continue
	})
	return hasClosed
}

// IsSymLink returns true if file is a symlink
func (fn *FileNode) IsSymLink() bool {
	return fn.Is(FileNodeSymLink)
}

// IsExec returns true if file is an executable file
func (fn *FileNode) IsExec() bool {
	return fn.Info.IsExec()
}

// IsOpen returns true if file is flagged as open
func (fn *FileNode) IsOpen() bool {
	return fn.Is(FileNodeOpen)
}

// SetOpen sets the open flag
func (fn *FileNode) SetOpen() {
	fn.SetFlag(true, FileNodeOpen)
}

// SetClosed clears the open flag
func (fn *FileNode) SetClosed() {
	fn.SetFlag(false, FileNodeOpen)
}

// IsChanged returns true if the file is open and has been changed (edited) since last save
func (fn *FileNode) IsChanged() bool {
	// if fn.Buf != nil && fn.Buf.IsChanged() {
	// 	return true
	// }
	return false
}

// IsAutoSave returns true if file is an auto-save file (starts and ends with #)
func (fn *FileNode) IsAutoSave() bool {
	if strings.HasPrefix(fn.Info.Name, "#") && strings.HasSuffix(fn.Info.Name, "#") {
		return true
	}
	return false
}

// MyRelPath returns the relative path from root for this node
func (fn *FileNode) MyRelPath() string {
	if fn.IsIrregular() {
		return fn.Nm
	}
	return RelFilePath(string(fn.FPath), string(fn.FRoot.FPath))
}

// ReadDir reads all the files at given directory into this directory node --
// uses config children to preserve extra info already stored about files. The
// root node represents the directory at the given path.  Returns os.Stat
// error if path cannot be accessed.
func (fn *FileNode) ReadDir(path string) error {
	_, fnm := filepath.Split(path)
	fn.SetName(fnm)
	pth, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	fn.FPath = gi.FileName(pth)
	err = fn.Info.InitFile(string(fn.FPath))
	if err != nil {
		log.Printf("giv.FileTree: could not read directory: %v err: %v\n", fn.FPath, err)
		return err
	}

	fn.UpdateDir()
	return nil
}

// DetectVcsRepo detects and configures DirRepo if this directory is root of
// a VCS repository.  if updateFiles is true, gets the files in the dir.
// returns true if a repository was newly found here.
func (fn *FileNode) DetectVcsRepo(updateFiles bool) bool {
	repo, _ := fn.Repo()
	if repo != nil {
		if updateFiles {
			fn.UpdateRepoFiles()
		}
		return false
	}
	path := string(fn.FPath)
	rtyp := vci.DetectRepo(path)
	if rtyp == vcs.NoVCS {
		return false
	}
	var err error
	repo, err = vci.NewRepo("origin", path)
	if err != nil {
		log.Println(err)
		return false
	}
	fn.DirRepo = repo
	if updateFiles {
		fn.UpdateRepoFiles()
	}
	return true
}

// UpdateDir updates the directory and all the nodes under it
func (fn *FileNode) UpdateDir() {
	fn.DetectVcsRepo(true) // update files
	path := string(fn.FPath)
	// fmt.Printf("path: %v  node: %v\n", path, fn.Path())
	repo, rnode := fn.Repo()
	fn.SetOpen()
	fn.FRoot.SetDirOpen(fn.FPath)
	config := fn.ConfigOfFiles(path)
	hasExtFiles := false
	if fn.This() == fn.FRoot.This() {
		if len(fn.FRoot.ExtFiles) > 0 {
			config = append([]ki.TypeAndName{{Type: fn.FRoot.NodeType, Name: FileTreeExtFilesName}}, config...)
			hasExtFiles = true
		}
	}
	mods, updt := fn.ConfigChildren(config) // NOT unique names
	if mods {
		// fmt.Printf("got mods: %v\n", path)
	}
	// always go through kids, regardless of mods
	for _, sfk := range fn.Kids {
		sf := AsFileNode(sfk)
		sf.FRoot = fn.FRoot
		if hasExtFiles && sf.Nm == FileTreeExtFilesName {
			fn.FRoot.UpdateExtFiles(sf)
			continue
		}
		fp := filepath.Join(path, sf.Nm)
		// if sf.Buf != nil {
		// 	fmt.Printf("fp: %v  nm: %v\n", fp, sf.Nm)
		// }
		sf.SetNodePath(fp)
		if sf.IsDir() {
			sf.Info.Vcs = vci.Stored // always
		} else if repo != nil {
			rstat := rnode.RepoFiles.Status(repo, string(sf.FPath))
			sf.Info.Vcs = rstat
		} else {
			sf.Info.Vcs = vci.Stored
		}
	}
	if mods {
		fn.UpdateEnd(updt)
	}
}

// ConfigOfFiles returns a type-and-name list for configuring nodes based on
// files immediately within given path
func (fn *FileNode) ConfigOfFiles(path string) ki.Config {
	config1 := ki.Config{}
	config2 := ki.Config{}
	typ := fn.FRoot.NodeType
	filepath.Walk(path, func(pth string, info os.FileInfo, err error) error {
		if err != nil {
			emsg := fmt.Sprintf("giv.FileNode ConfigFilesIn Path %q: Error: %v", path, err)
			log.Println(emsg)
			return nil // ignore
		}
		if pth == path { // proceed..
			return nil
		}
		_, fnm := filepath.Split(pth)
		if fn.FRoot.DirsOnTop {
			if info.IsDir() {
				config1.Add(typ, fnm)
			} else {
				config2.Add(typ, fnm)
			}
		} else {
			config1.Add(typ, fnm)
		}
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	modSort := fn.FRoot.DirSortByModTime(gi.FileName(path))
	if fn.FRoot.DirsOnTop {
		if modSort {
			fn.SortConfigByModTime(config2) // just sort files, not dirs
		}
		config1 = append(config1, config2...)
	} else {
		if modSort {
			fn.SortConfigByModTime(config1) // all
		}
	}
	return config1
}

// SortConfigByModTime sorts given config list by mod time
func (fn *FileNode) SortConfigByModTime(confg ki.Config) {
	sort.Slice(confg, func(i, j int) bool {
		ifn, _ := os.Stat(filepath.Join(string(fn.FPath), confg[i].Name))
		jfn, _ := os.Stat(filepath.Join(string(fn.FPath), confg[j].Name))
		return ifn.ModTime().After(jfn.ModTime()) // descending
	})
}

// SetNodePath sets the path for given node and updates it based on associated file
func (fn *FileNode) SetNodePath(path string) error {
	pth, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	fn.FPath = gi.FileName(pth)
	err = fn.InitFileInfo()
	if err != nil {
		return err
	}
	if fn.IsDir() && !fn.IsIrregular() {
		openAll := fn.FRoot.InOpenAll && !fn.Info.IsHidden()
		if openAll || fn.FRoot.IsDirOpen(fn.FPath) {
			fn.ReadDir(string(fn.FPath)) // keep going down..
		}
	}
	return nil
}

// InitFileInfo initializes file info
func (fn *FileNode) InitFileInfo() error {
	effpath, err := filepath.EvalSymlinks(string(fn.FPath))
	if err != nil {
		// this happens too often for links -- skip
		// log.Printf("giv.FileNode Path: %v could not be opened -- error: %v\n", fn.FPath, err)
		return err
	}
	fn.FPath = gi.FileName(effpath)
	err = fn.Info.InitFile(string(fn.FPath))
	if err != nil {
		emsg := fmt.Errorf("giv.FileNode InitFileInfo Path %q: Error: %v", fn.FPath, err)
		log.Println(emsg)
		return emsg
	}
	return nil
}

// UpdateNode updates information in node based on its associated file in FPath.
// This is intended to be called ad-hoc for individual nodes that might need
// updating -- use ReadDir for mass updates as it is more efficient.
func (fn *FileNode) UpdateNode() error {
	err := fn.InitFileInfo()
	if err != nil {
		return err
	}
	if fn.IsIrregular() {
		return nil
	}
	if fn.IsDir() {
		openAll := fn.FRoot.InOpenAll && !fn.Info.IsHidden()
		if openAll || fn.FRoot.IsDirOpen(fn.FPath) {
			// fmt.Printf("set open: %s\n", fn.FPath)
			fn.SetOpen()
			fn.FRoot.SetDirOpen(fn.FPath)
			repo, rnode := fn.Repo()
			if repo != nil {
				rnode.UpdateRepoFiles()
			}
			fn.UpdateDir()
		}
	} else {
		repo, _ := fn.Repo()
		if repo != nil {
			fn.Info.Vcs, _ = repo.Status(string(fn.FPath))
		}
		// fn.UpdateSig()
		// fn.FRoot.UpdateSig()
	}
	return nil
}

// OpenDir opens given directory node
func (fn *FileNode) OpenDir() {
	// fmt.Printf("fn: %s opened\n", fn.FPath)
	fn.SetOpen()
	fn.FRoot.SetDirOpen(fn.FPath)
	fn.UpdateNode()
}

// CloseDir closes given directory node -- updates memory state
func (fn *FileNode) CloseDir() {
	// fmt.Printf("fn: %s closed\n", fn.FPath)
	fn.FRoot.SetDirClosed(fn.FPath)
	fn.SetClosed()
	// note: not doing anything with open files within directory..
}

// SortBy determines how to sort the files in the directory -- default is alpha by name,
// optionally can be sorted by modification time.
func (fn *FileNode) SortBy(modTime bool) {
	fn.FRoot.SetDirSortBy(fn.FPath, modTime)
	fn.UpdateNode()
	// todo:
	// fn.UpdateSig() // make sure
}

// OpenAll opens all directories under this one
func (fn *FileNode) OpenAll() {
	fn.FRoot.InOpenAll = true
	fn.SetOpen()
	fn.FRoot.SetDirOpen(fn.FPath)
	fn.UpdateNode()
	fn.FRoot.InOpenAll = false
	// note: FileTreeView must actually do the open all too!
}

// CloseAll closes all directories under this one, this included
func (fn *FileNode) CloseAll() {
	fn.SetClosed()
	fn.FRoot.SetDirClosed(fn.FPath)
	fn.WalkPre(func(k ki.Ki) bool {
		sfn := AsFileNode(k)
		if sfn.IsDir() {
			sfn.SetClosed()
			sfn.FRoot.SetDirClosed(sfn.FPath)
		}
		return ki.Continue
	})
	// note: FileTreeView must actually do the close all too!
}

// OpenBuf opens the file in its buffer if it is not already open.
// returns true if file is newly opened
func (fn *FileNode) OpenBuf() (bool, error) {
	if fn.IsDir() {
		err := fmt.Errorf("giv.FileNode cannot open directory in editor: %v", fn.FPath)
		log.Println(err.Error())
		return false, err
	}
	if fn.Buf != nil {
		if fn.Buf.Filename == fn.FPath { // close resets filename
			return false, nil
		}
	} else {
		fn.Buf = &TextBuf{}
		fn.Buf.InitName(fn.Buf, fn.Nm)
		// fn.Buf.AddFileNode(fn)
	}
	fn.Buf.Hi.Style = FileNodeHiStyle
	// return true, fn.Buf.Open(fn.FPath)
	return true, nil
}

// CloseBuf closes the file in its buffer if it is open -- returns true if closed
// Connect to the fn.Buf.TextBufSig and look for the TextBufClosed signal to be
// notified when the buffer is closed.
func (fn *FileNode) CloseBuf() bool {
	if fn.Buf == nil {
		return false
	}
	// fn.Buf.Close(nil)
	// fn.Buf = nil
	fn.SetClosed()
	return true
}

// FindDirNode finds directory node by given path -- must be a relative
// path already rooted at tree, or absolute path within tree
func (fn *FileNode) FindDirNode(path string) (*FileNode, error) {
	rp := fn.RelPath(gi.FileName(path))
	if rp == "" {
		return nil, fmt.Errorf("FindDirNode: path: %s is not relative to this node's path: %s", path, fn.FPath)
	}
	if rp == "." {
		return fn, nil
	}
	dirs := filepath.SplitList(rp)
	dir := dirs[0]
	dni, err := fn.ChildByNameTry(dir, 0)
	if err != nil {
		return nil, err
	}
	dn := AsFileNode(dni)
	if len(dirs) == 1 {
		if dn.IsDir() {
			return dn, nil
		}
		return nil, fmt.Errorf("FindDirNode: item at path: %s is not a Directory", path)
	}
	return dn.FindDirNode(filepath.Join(dirs[1:]...))
}

// RelPath returns the relative path from node for given full path
func (fn *FileNode) RelPath(fpath gi.FileName) string {
	return RelFilePath(string(fpath), string(fn.FPath))
}

// DirsTo opens all the directories above the given filename, and returns the node
// for element at given path (can be a file or directory itself -- not opened -- just returned)
func (fn *FileNode) DirsTo(path string) (*FileNode, error) {
	pth, err := filepath.Abs(path)
	if err != nil {
		log.Printf("giv.FileNode DirsTo path %v could not be turned into an absolute path: %v\n", path, err)
		return nil, err
	}
	rpath := fn.RelPath(gi.FileName(pth))
	if rpath == "." {
		return fn, nil
	}
	dirs := strings.Split(rpath, string(filepath.Separator))
	cfn := fn
	sz := len(dirs)
	for i := 0; i < sz; i++ {
		dr := dirs[i]
		sfni, err := cfn.ChildByNameTry(dr, 0)
		if err != nil {
			if i == sz-1 { // ok for terminal -- might not exist yet
				return cfn, nil
			} else {
				err = fmt.Errorf("giv.FileNode could not find node %v in: %v", dr, cfn.FPath)
				// log.Println(err)
				return nil, err
			}
		}
		sfn := AsFileNode(sfni)
		if sfn.IsDir() || i == sz-1 {
			if i < sz-1 && !sfn.IsOpen() {
				sfn.OpenDir()
				sfn.UpdateNode()
			} else {
				cfn = sfn
			}
		} else {
			err := fmt.Errorf("giv.FileNode non-terminal node %v is not a directory in: %v", dr, cfn.FPath)
			log.Println(err)
			return nil, err
		}
		cfn = sfn
	}
	return cfn, nil
}

// FindFile finds first node representing given file (false if not found) --
// looks for full path names that have the given string as their suffix, so
// you can include as much of the path (including whole thing) as is relevant
// to disambiguate.  See FilesMatching for a list of files that match a given
// string.
func (fn *FileNode) FindFile(fnm string) (*FileNode, bool) {
	if fnm == "" {
		return nil, false
	}
	fneff := fnm
	if len(fneff) > 2 && fneff[:2] == ".." { // relative path -- get rid of it and just look for relative part
		dirs := strings.Split(fneff, string(filepath.Separator))
		for i, dr := range dirs {
			if dr != ".." {
				fneff = filepath.Join(dirs[i:]...)
				break
			}
		}
	}

	if efn, err := fn.FRoot.ExtFileNodeByPath(fnm); err == nil {
		return efn, true
	}

	if strings.HasPrefix(fneff, string(fn.FPath)) { // full path
		ffn, err := fn.DirsTo(fneff)
		if err == nil {
			return ffn, true
		}
		return nil, false
	}

	var ffn *FileNode
	found := false
	fn.WalkPre(func(k ki.Ki) bool {
		sfn := AsFileNode(k)
		if strings.HasSuffix(string(sfn.FPath), fneff) {
			ffn = sfn
			found = true
			return ki.Break
		}
		return ki.Continue
	})
	return ffn, found
}

// FilesMatching returns list of all nodes whose file name contains given
// string (no regexp) -- ignoreCase transforms everything into lowercase
func (fn *FileNode) FilesMatching(match string, ignoreCase bool) []*FileNode {
	mls := make([]*FileNode, 0)
	if ignoreCase {
		match = strings.ToLower(match)
	}
	fn.WalkPre(func(k ki.Ki) bool {
		sfn := AsFileNode(k)
		if ignoreCase {
			nm := strings.ToLower(sfn.Nm)
			if strings.Contains(nm, match) {
				mls = append(mls, sfn)
			}
		} else {
			if strings.Contains(sfn.Nm, match) {
				mls = append(mls, sfn)
			}
		}
		return ki.Continue
	})
	return mls
}

// FileNodeNameCount is used to report counts of different string-based things
// in the file tree
type FileNodeNameCount struct {
	Name  string
	Count int
}

func FileNodeNameCountSort(ecs []FileNodeNameCount) {
	sort.Slice(ecs, func(i, j int) bool {
		return ecs[i].Count > ecs[j].Count
	})
}

// FirstVCS returns the first VCS repository starting from this node and going down.
// also returns the node having that repository
func (fn *FileNode) FirstVCS() (vci.Repo, *FileNode) {
	var repo vci.Repo
	var rnode *FileNode
	fn.WalkPre(func(k ki.Ki) bool {
		sfn := AsFileNode(k)
		if sfn.DirRepo != nil {
			repo = sfn.DirRepo
			rnode = sfn
			return ki.Break
		}
		return ki.Continue
	})
	return repo, rnode
}

// FileExtCounts returns a count of all the different file extensions, sorted
// from highest to lowest.
// If cat is != filecat.Unknown then it only uses files of that type
// (e.g., filecat.Code to find any code files)
func (fn *FileNode) FileExtCounts(cat filecat.Cat) []FileNodeNameCount {
	cmap := make(map[string]int, 20)
	fn.WalkPre(func(k ki.Ki) bool {
		sfn := AsFileNode(k)
		if cat != filecat.Unknown {
			if sfn.Info.Cat != cat {
				return ki.Continue
			}
		}
		ext := strings.ToLower(filepath.Ext(sfn.Nm))
		if ec, has := cmap[ext]; has {
			cmap[ext] = ec + 1
		} else {
			cmap[ext] = 1
		}
		return ki.Continue
	})
	ecs := make([]FileNodeNameCount, len(cmap))
	idx := 0
	for key, val := range cmap {
		ecs[idx] = FileNodeNameCount{Name: key, Count: val}
		idx++
	}
	FileNodeNameCountSort(ecs)
	return ecs
}

// LatestFileMod returns the most recent mod time of files in the tree.
// If cat is != filecat.Unknown then it only uses files of that type
// (e.g., filecat.Code to find any code files)
func (fn *FileNode) LatestFileMod(cat filecat.Cat) time.Time {
	tmod := time.Time{}
	fn.WalkPre(func(k ki.Ki) bool {
		sfn := AsFileNode(k)
		if cat != filecat.Unknown {
			if sfn.Info.Cat != cat {
				return ki.Continue
			}
		}
		ft := (time.Time)(sfn.Info.ModTime)
		if ft.After(tmod) {
			tmod = ft
		}
		return ki.Continue
	})
	return tmod
}

//////////////////////////////////////////////////////////////////////////////
//    File ops

// OSOpenCommand returns the generic file 'open' command to open file with default app
// open on Mac, xdg-open on Linux, and start on RenderWins
func OSOpenCommand() string {
	switch goosi.TheApp.Platform() {
	case goosi.MacOS:
		return "open"
	case goosi.LinuxX11:
		return "xdg-open"
	case goosi.Windows:
		return "start"
	}
	return "open"
}

// OpenFileDefault opens file with default app for that file type (os defined)
// runs open on Mac, xdg-open on Linux, and start on RenderWins
func (fn *FileNode) OpenFileDefault() error {
	cstr := OSOpenCommand()
	cmd := exec.Command(cstr, string(fn.FPath))
	out, err := cmd.CombinedOutput()
	fmt.Printf("%s\n", out)
	return err
}

// OpenFileWith opens file with given command.
// does not wait for command to finish in this routine (separate routine Waits)
func (fn *FileNode) OpenFileWith(command string) error {
	cmd := exec.Command(command, string(fn.FPath))
	err := cmd.Start()
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Println(err)
		}
	}()
	return err
}

// Duplicate creates a copy of given file -- only works for regular files, not
// directories
func (fn *FileNode) DuplicateFile() error {
	_, err := fn.Info.Duplicate()
	if err == nil && fn.Par != nil {
		fnp := AsFileNode(fn.Par)
		fnp.UpdateNode()
	}
	return err
}

// DeleteFile deletes this file
func (fn *FileNode) DeleteFile() (err error) {
	if fn.IsExternal() {
		return nil
	}
	fn.CloseBuf()
	repo, _ := fn.Repo()
	if !fn.Info.IsDir() && repo != nil && fn.Info.Vcs >= vci.Stored {
		// fmt.Printf("del repo: %v\n", fn.FPath)
		err = repo.Delete(string(fn.FPath))
	} else {
		// fmt.Printf("del raw: %v\n", fn.FPath)
		err = fn.Info.Delete()
	}
	if err == nil {
		fn.Delete(true)
	}
	return err
}

// RenameFile renames file to new name
func (fn *FileNode) RenameFile(newpath string) (err error) {
	if fn.IsExternal() {
		return nil
	}
	fn.CloseBuf() // invalid after this point
	orgpath := fn.FPath
	newpath, err = fn.Info.Rename(newpath)
	if len(newpath) == 0 || err != nil {
		return err
	}
	if fn.IsDir() {
		if fn.FRoot.IsDirOpen(orgpath) {
			fn.FRoot.SetDirOpen(gi.FileName(newpath))
		}
	}
	repo, _ := fn.Repo()
	stored := false
	if fn.IsDir() && !fn.HasChildren() {
		err = os.Rename(string(orgpath), newpath)
	} else if repo != nil && fn.Info.Vcs >= vci.Stored {
		stored = true
		err = repo.Move(string(orgpath), newpath)
	} else {
		err = os.Rename(string(orgpath), newpath)
	}
	if err == nil {
		err = fn.Info.InitFile(newpath)
	}
	if err == nil {
		fn.FPath = gi.FileName(fn.Info.Path)
		fn.SetName(fn.Info.Name)
	}
	if stored {
		fn.AddToVcs()
	} else {
		// fn.UpdateSig() // todo
		fn.FRoot.UpdateDir() // need full update
	}
	return err
}

// NewFile makes a new file in given selected directory node
func (fn *FileNode) NewFile(filename string, addToVcs bool) {
	if fn.IsExternal() {
		return
	}
	ppath := string(fn.FPath)
	if !fn.IsDir() {
		ppath, _ = filepath.Split(ppath)
	}
	np := filepath.Join(ppath, filename)
	_, err := os.Create(np)
	if err != nil {
		gi.PromptDialog(nil, gi.DlgOpts{Title: "Couldn't Make File", Prompt: fmt.Sprintf("Could not make new file at: %v, err: %v", np, err), Ok: true, Cancel: false}, nil)
		return
	}
	fn.FRoot.UpdateNewFile(np)
	if addToVcs {
		nfn, ok := fn.FRoot.FindFile(np)
		if ok && nfn.This() != fn.FRoot.This() {
			nfn.AddToVcs()
		}
	}
}

// NewFolder makes a new folder (directory) in given selected directory node
func (fn *FileNode) NewFolder(foldername string) {
	if fn.IsExternal() {
		return
	}
	ppath := string(fn.FPath)
	if !fn.IsDir() {
		ppath, _ = filepath.Split(ppath)
	}
	np := filepath.Join(ppath, foldername)
	err := os.MkdirAll(np, 0775)
	if err != nil {
		emsg := fmt.Sprintf("giv.FileNode at: %q: Error: %v", ppath, err)
		gi.PromptDialog(nil, gi.DlgOpts{Title: "Couldn't Make Folder", Prompt: emsg, Ok: true, Cancel: false}, nil)
		return
	}
	fn.FRoot.UpdateNewFile(ppath)
}

// CopyFileToDir copies given file path into node that is a directory.
// This does NOT check for overwriting -- that must be done at higher level!
func (fn *FileNode) CopyFileToDir(filename string, perm os.FileMode) {
	if fn.IsExternal() {
		return
	}
	ppath := string(fn.FPath)
	sfn := filepath.Base(filename)
	tpath := filepath.Join(ppath, sfn)
	CopyFile(tpath, filename, perm)
	fn.FRoot.UpdateNewFile(ppath)
	ofn, ok := fn.FRoot.FindFile(filename)
	if ok && ofn.Info.Vcs >= vci.Stored {
		nfn, ok := fn.FRoot.FindFile(tpath)
		if ok && nfn.This() != fn.FRoot.This() {
			nfn.AddToVcs()
			nfn.UpdateNode()
		}
	}
}

//////////////////////////////////////////////////////////////////////////////
//    File VCS ops

// Repo returns the version control repository associated with this file,
// and the node for the directory where the repo is based.
// Goes up the tree until a repository is found.
func (fn *FileNode) Repo() (vci.Repo, *FileNode) {
	if fn.IsExternal() {
		return nil, nil
	}
	if fn.DirRepo != nil {
		return fn.DirRepo, fn
	}
	var repo vci.Repo
	var rnode *FileNode
	fn.WalkUpParent(func(k ki.Ki) bool {
		sfn := AsFileNode(k)
		if sfn.IsIrregular() {
			return ki.Break
		}
		if sfn.DirRepo != nil {
			repo = sfn.DirRepo
			rnode = sfn
			return ki.Break
		}
		return ki.Continue
	})
	return repo, rnode
}

func (fn *FileNode) UpdateRepoFiles() {
	if fn.DirRepo == nil {
		return
	}
	fn.RepoFiles, _ = fn.DirRepo.Files()
}

// AddToVcs adds file to version control
func (fn *FileNode) AddToVcs() {
	repo, _ := fn.Repo()
	if repo == nil {
		return
	}
	// fmt.Printf("adding to vcs: %v\n", fn.FPath)
	err := repo.Add(string(fn.FPath))
	if err == nil {
		fn.Info.Vcs = vci.Added
		// fn.UpdateSig()
		// fn.FRoot.UpdateSig()
		return
	}
	fmt.Println(err)
}

// DeleteFromVcs removes file from version control
func (fn *FileNode) DeleteFromVcs() {
	repo, _ := fn.Repo()
	if repo == nil {
		return
	}
	// fmt.Printf("deleting remote from vcs: %v\n", fn.FPath)
	err := repo.DeleteRemote(string(fn.FPath))
	if fn != nil && err == nil {
		fn.Info.Vcs = vci.Deleted
		// fn.UpdateSig()
		// fn.FRoot.UpdateSig()
		return
	}
	fmt.Println(err)
}

// CommitToVcs commits file changes to version control system
func (fn *FileNode) CommitToVcs(message string) (err error) {
	repo, _ := fn.Repo()
	if repo == nil {
		return
	}
	if fn.Info.Vcs == vci.Untracked {
		return errors.New("file not in vcs repo: " + string(fn.FPath))
	}
	err = repo.CommitFile(string(fn.FPath), message)
	if err != nil {
		return err
	}
	fn.Info.Vcs = vci.Stored
	// fn.UpdateSig()
	// fn.FRoot.UpdateSig()
	return err
}

// RevertVcs reverts file changes since last commit
func (fn *FileNode) RevertVcs() (err error) {
	repo, _ := fn.Repo()
	if repo == nil {
		return
	}
	if fn.Info.Vcs == vci.Untracked {
		return errors.New("file not in vcs repo: " + string(fn.FPath))
	}
	err = repo.RevertFile(string(fn.FPath))
	if err != nil {
		return err
	}
	if fn.Info.Vcs == vci.Modified {
		fn.Info.Vcs = vci.Stored
	} else if fn.Info.Vcs == vci.Added {
		// do nothing - leave in "added" state
	}
	// if fn.Buf != nil {
	// 	fn.Buf.Revert()
	// }
	// fn.UpdateSig()
	// fn.FRoot.UpdateSig()
	return err
}

// DiffVcs shows the diffs between two versions of this file, given by the
// revision specifiers -- if empty, defaults to A = current HEAD, B = current WC file.
// -1, -2 etc also work as universal ways of specifying prior revisions.
// Diffs are shown in a DiffViewDialog.
func (fn *FileNode) DiffVcs(rev_a, rev_b string) error {
	repo, _ := fn.Repo()
	if repo == nil {
		return errors.New("file not in vcs repo: " + string(fn.FPath))
	}
	if fn.Info.Vcs == vci.Untracked {
		return errors.New("file not in vcs repo: " + string(fn.FPath))
	}
	// _, err := DiffViewDialogFromRevs(nil, repo, string(fn.FPath), fn.Buf, rev_a, rev_b)
	// return err
	return nil
}

// LogVcs shows the VCS log of commits for this file, optionally with a
// since date qualifier: If since is non-empty, it should be
// a date-like expression that the VCS will understand, such as
// 1/1/2020, yesterday, last year, etc.  SVN only understands a
// number as a maximum number of items to return.
// If allFiles is true, then the log will show revisions for all files, not just
// this one.
// Returns the Log and also shows it in a VCSLogView which supports further actions.
func (fn *FileNode) LogVcs(allFiles bool, since string) (vci.Log, error) {
	repo, _ := fn.Repo()
	if repo == nil {
		return nil, errors.New("file not in vcs repo: " + string(fn.FPath))
	}
	if fn.Info.Vcs == vci.Untracked {
		return nil, errors.New("file not in vcs repo: " + string(fn.FPath))
	}
	fnm := string(fn.FPath)
	if allFiles {
		fnm = ""
	}
	lg, err := repo.Log(fnm, since)
	if err != nil {
		return lg, err
	}
	VCSLogViewDialog(nil, repo, lg, fnm, since)
	return lg, nil
}

// BlameDialog opens a dialog for displaying VCS blame data using TwinTextViews.
// blame is the annotated blame code, while fbytes is the original file contents.
func BlameDialog(ctx gi.Widget, fname string, blame, fbytes []byte) *TwinTextViews {
	title := "VCS Blame: " + DirAndFile(fname)
	dlg := gi.NewStdDialog(ctx, gi.DlgOpts{Title: title, Ok: true, Cancel: false}, nil)

	frame := dlg.Stage.Scene
	prIdx := dlg.PromptWidgetIdx()

	tv := frame.InsertNewChild(TwinTextViewsType, prIdx+1, "twin-view").(*TwinTextViews)
	tv.SetStretchMax()
	tv.SetFiles(fname, fname, true)
	flns := bytes.Split(fbytes, []byte("\n"))
	lns := bytes.Split(blame, []byte("\n"))
	nln := min(len(lns), len(flns))
	blns := make([][]byte, nln)
	stidx := 0
	for i := 0; i < nln; i++ {
		fln := flns[i]
		bln := lns[i]
		if stidx == 0 {
			if len(fln) == 0 {
				stidx = len(bln)
			} else {
				stidx = bytes.LastIndex(bln, fln)
			}
		}
		blns[i] = bln[:stidx]
	}
	btxt := bytes.Join(blns, []byte("\n")) // makes a copy, so blame is disposable now
	tv.BufA.SetText(btxt)
	tv.BufB.SetText(fbytes)
	tv.ConfigTexts()
	tv.SetSplits(.2, .8)

	tva, tvb := tv.TextViews()
	tva.AddStyles(func(s *styles.Style) {
		s.Text.WhiteSpace = styles.WhiteSpacePre
		s.Width.SetCh(30)
		s.Height.SetEm(40)
	})
	tvb.AddStyles(func(s *styles.Style) {
		s.Text.WhiteSpace = styles.WhiteSpacePre
		s.Width.SetCh(80)
		s.Height.SetEm(40)
	})

	// dlg.UpdateEndNoSig(true) // going to be shown
	// dlg.Open(0, 0, avp, nil)
	dlg.Run()
	return tv
}

// BlameVcs shows the VCS blame report for this file, reporting for each line
// the revision and author of the last change.
func (fn *FileNode) BlameVcs() ([]byte, error) {
	repo, _ := fn.Repo()
	if repo == nil {
		return nil, errors.New("file not in vcs repo: " + string(fn.FPath))
	}
	if fn.Info.Vcs == vci.Untracked {
		return nil, errors.New("file not in vcs repo: " + string(fn.FPath))
	}
	fnm := string(fn.FPath)
	fb, err := textbuf.FileBytes(fnm)
	if err != nil {
		return nil, err
	}
	blm, err := repo.Blame(fnm)
	if err != nil {
		return blm, err
	}
	BlameDialog(nil, fnm, blm, fb)
	return blm, nil
}

// UpdateAllVcs does an update on any repositories below this one in file tree
func (fn *FileNode) UpdateAllVcs() {
	fn.WalkPre(func(k ki.Ki) bool {
		sfn := AsFileNode(k)
		if !sfn.IsDir() {
			return ki.Continue
		}
		if sfn.DirRepo == nil {
			if !sfn.DetectVcsRepo(false) {
				return ki.Continue
			}
		}
		repo := sfn.DirRepo
		fmt.Printf("Updating %v repository: %s from: %s\n", repo.Vcs(), sfn.MyRelPath(), repo.Remote())
		err := repo.Update()
		if err != nil {
			fmt.Printf("error: %v\n", err)
		}
		return ki.Break
	})
}

var FileNodeProps = ki.Props{
	"CallMethods": ki.PropSlice{
		{"RenameFile", ki.Props{
			"label": "Rename...",
			"desc":  "Rename file to new file name",
			"Args": ki.PropSlice{
				{"New Name", ki.Props{
					"width":         60,
					"default-field": "Nm",
				}},
			},
		}},
		{"OpenFileWith", ki.Props{
			"label": "Open With...",
			"desc":  "Open the file with given command...",
			"Args": ki.PropSlice{
				{"Command", ki.Props{
					"width": 60,
				}},
			},
		}},
		{"NewFile", ki.Props{
			"label": "New File...",
			"desc":  "Create a new file in this folder",
			"Args": ki.PropSlice{
				{"File Name", ki.Props{
					"width": 60,
				}},
				{"Add To Version Control", ki.Props{}},
			},
		}},
		{"NewFolder", ki.Props{
			"label": "New Folder...",
			"desc":  "Create a new folder within this folder",
			"Args": ki.PropSlice{
				{"Folder Name", ki.Props{
					"width": 60,
				}},
			},
		}},
		{"CommitToVcs", ki.Props{
			"label": "Commit to Vcs...",
			"desc":  "Commit this file to version control",
			"Args": ki.PropSlice{
				{"Message", ki.Props{
					"width": 60,
				}},
			},
		}},
	},
}

//////////////////////////////////////////////////////////////////////////////
//    FileNodeFlags

// FileNodeFlags define bitflags for FileNode state -- these extend ki.Flags
// and storage is an int64
type FileNodeFlags ki.Flags //enums:bitflag

const (
	// FileNodeOpen means file is open -- for directories, this means that
	// sub-files should be / have been loaded -- for files, means that they
	// have been opened e.g., for editing
	FileNodeOpen FileNodeFlags = FileNodeFlags(ki.FlagsN) + iota

	// FileNodeSymLink indicates that file is a symbolic link -- file info is
	// all for the target of the symlink
	FileNodeSymLink
)

//////////////////////////////////////////////////////////////////////////////
//    DirFlagMap

// DirFlags are flags on directories: Open, SortBy etc
// These flags are stored in the DirFlagMap for persistence.
type DirFlags int64 //enums:bitflag

const (
	// DirMark means directory is marked -- unmarked entries are deleted post-update
	DirMark DirFlags = iota

	// DirIsOpen means directory is open -- else closed
	DirIsOpen

	// DirSortByName means sort the directory entries by name.
	// this is mutex with other sorts -- keeping option open for non-binary sort choices.
	DirSortByName

	// DirSortByModTime means sort the directory entries by modification time
	DirSortByModTime
)

// DirFlagMap is a map for encoding directories that are open in the file
// tree.  The strings are typically relative paths.  The bool value is used to
// mark active paths and inactive (unmarked) ones can be removed.
// Map access is protected by Mutex.
type DirFlagMap struct {

	// map of paths and associated flags
	Map map[string]DirFlags `desc:"map of paths and associated flags"`

	// [view: -] mutex for accessing map
	Mu sync.Mutex `view:"-" json:"-" xml:"-" desc:"mutex for accessing map"`
}

// Init initializes the map, and sets the Mutex lock -- must unlock manually
func (dm *DirFlagMap) Init() {
	dm.Mu.Lock()
	if dm.Map == nil {
		dm.Map = make(map[string]DirFlags)
	}
}

// IsOpen returns true if path has IsOpen bit flag set
func (dm *DirFlagMap) IsOpen(path string) bool {
	dm.Init()
	defer dm.Mu.Unlock()
	if df, ok := dm.Map[path]; ok {
		return df.HasFlag(DirIsOpen)
	}
	return false
}

// SetOpenState sets the given directory's open flag
func (dm *DirFlagMap) SetOpen(path string, open bool) {
	dm.Init()
	defer dm.Mu.Unlock()
	df := dm.Map[path]
	df.SetFlag(open, DirIsOpen)
	dm.Map[path] = df
}

// SortByName returns true if path is sorted by name (default if not in map)
func (dm *DirFlagMap) SortByName(path string) bool {
	dm.Init()
	defer dm.Mu.Unlock()
	if df, ok := dm.Map[path]; ok {
		return df.HasFlag(DirSortByName)
	}
	return true
}

// SortByModTime returns true if path is sorted by mod time
func (dm *DirFlagMap) SortByModTime(path string) bool {
	dm.Init()
	defer dm.Mu.Unlock()
	if df, ok := dm.Map[path]; ok {
		return df.HasFlag(DirSortByModTime)
	}
	return false
}

// SetSortBy sets the given directory's sort by option
func (dm *DirFlagMap) SetSortBy(path string, modTime bool) {
	dm.Init()
	defer dm.Mu.Unlock()
	df := dm.Map[path]
	// mask := bitflag.Mask32(int(DirSortByName), int(DirSortByModTime))
	// bitflag.ClearMask32((*int32)(&df), mask)
	// if modTime {
	// 	bitflag.Set32((*int32)(&df), int(DirSortByModTime))
	// } else {
	// 	bitflag.Set32((*int32)(&df), int(DirSortByName))
	// }
	dm.Map[path] = df
}

// SetMark sets the mark flag indicating we visited file
func (dm *DirFlagMap) SetMark(path string) {
	dm.Init()
	defer dm.Mu.Unlock()
	df := dm.Map[path]
	// bitflag.Set32((*int32)(&df), int(DirMark))
	dm.Map[path] = df
}

// ClearMarks clears all the marks -- do this prior to traversing
// full set of active paths -- can then call DeleteStale to get rid of unused paths.
func (dm *DirFlagMap) ClearMarks() {
	dm.Init()
	defer dm.Mu.Unlock()
	for key, df := range dm.Map {
		// bitflag.Clear32((*int32)(&df), int(DirMark))
		dm.Map[key] = df
	}
}

// DeleteStale removes all entries with a bool = false value indicating that
// they have not been accessed since ClearFlags was called.
func (dm *DirFlagMap) DeleteStale() {
	dm.Init()
	defer dm.Mu.Unlock()
	for key, df := range dm.Map {
		if !df.HasFlag(DirMark) {
			delete(dm.Map, key)
		}
	}
}

//////////////////////////////////////////////////////////////////////////////
//    FileTreeView

// FileTreeView is a TreeView that knows how to operate on FileNode nodes
//
//goki:embedder
type FileTreeView struct {
	TreeView
}

// exists for same reason as TreeView one (init cycle)
func init() {
	// kit.Types.SetProps(TypeFileTreeView, FileTreeViewProps)
}

func (ftv *FileTreeView) OnInit() {
	ftv.SetFlag(true, TreeViewFlagUpdtRoot) // filetree needs this
	ftv.OpenDepth = 4
	ftv.Indent.SetEm(1)

	ftv.HandleFileTreeViewEvents()
	ftv.FileTreeViewStyles()
}

func (ftv *FileTreeView) FileTreeViewStyles() {

	ftv.AddStyles(func(s *styles.Style) {
		s.Border.Style.Set(styles.BorderNone)
		s.Border.Radius.Set()
		s.Margin.Set()
		s.Padding.Set()
		s.Text.Align = styles.AlignLeft
		s.AlignV = styles.AlignTop
		if ftv.StateIs(states.Selected) {
			s.BackgroundColor.SetSolid(colors.Scheme.Tertiary.Container)
		}
		switch ftv.Class {
		case "exec":
			s.Font.Weight = styles.WeightBold
		case "open":
			s.Font.Style = styles.FontItalic
		case "untracked":
			s.Color = grr.Must(colors.FromHex("#808080"))
		case "modified":
			s.Color = grr.Must(colors.FromHex("#4b7fd1"))
		case "added":
			s.Color = grr.Must(colors.FromHex("#008800"))
		case "deleted":
			s.Color = grr.Must(colors.FromHex("#ff4252"))
		case "conflicted":
			s.Color = grr.Must(colors.FromHex("#ce8020"))
		case "updated":
			s.Color = grr.Must(colors.FromHex("#008060"))
		}
	})
}

func (ftv *FileTreeView) OnChildAdded(child ki.Ki) {
	w, _ := gi.AsWidget(child)
	switch w.Name() {
	case "Parts":
		parts := w.(*gi.Layout)
		parts.AddStyles(func(s *styles.Style) {
			parts.Spacing.SetCh(0.5)
		})
	case "icon":
		w.AddStyles(func(s *styles.Style) {
			s.Width.SetEm(1)
			s.Height.SetEm(1)
			s.Margin.Set()
			s.Padding.Set()
		})
	case "branch":
		cb := w.(*gi.Switch)
		cb.IconOn = icons.FolderOpen
		cb.IconOff = icons.Folder
		cb.AddStyles(func(s *styles.Style) {
			s.Margin.Set()
			s.Padding.Set()
			s.MaxWidth.SetEm(1.5)
			s.MaxHeight.SetEm(1.5)
			s.AlignV = styles.AlignMiddle
		})
	case "space":
		w.AddStyles(func(s *styles.Style) {
			s.Width.SetEm(0.5)
		})
	case "label":
		w.AddStyles(func(s *styles.Style) {
			s.Margin.Set()
			s.Padding.Set()
			s.MinWidth.SetCh(16)
		})
	case "menu":
		menu := w.(*gi.Button)
		menu.Indicator = icons.None
	}
}

// FileNode returns the SrcNode as a FileNode
func (ftv *FileTreeView) FileNode() *FileNode {
	if ftv.This() == nil {
		return nil
	}
	return AsFileNode(ftv.SrcNode)
}

func (ftv *FileTreeView) UpdateAllFiles() {
	fn := ftv.FileNode()
	if fn != nil {
		fn.FRoot.UpdateAll()
		ftv.RootView.ReSync() // manual resync
	}
}

func (ftv *FileTreeView) HandleFileTreeViewEvents() {
	ftv.On(events.KeyChord, func(e events.Event) {
		kt := e.(*events.Key)
		ftv.KeyInput(kt)
	})
	/*
		ftvwe.AddFunc(goosi.DNDEvent, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
			de := d.(events.Event)
			tvvi := recv.Embed(TypeFileTreeView)
			if tvvi == nil {
				return
			}
			tvv := tvvi.(*FileTreeView)
			switch de.Action {
			case events.Start:
				tvv.DragNDropStart()
			case events.DropOnTarget:
				tvv.DragNDropTarget(de)
			case events.DropFmSource:
				tvv.This().(gi.DragNDropper).Dragged(de)
			case events.External:
				tvv.DragNDropExternal(de)
			}
		})
		ftvwe.AddFunc(goosi.DNDFocusEvent, gi.RegPri, func(recv, send ki.Ki, sig int64, d any) {
			de := d.(*events.FocusEvent)
			tvvi := recv.Embed(TypeFileTreeView)
			if tvvi == nil {
				return
			}
			tvv := tvvi.(*FileTreeView)
			switch de.Action {
			case events.Enter:
				tvv.ParentRenderWin().DNDSetCursor(de.Mod)
			case events.Exit:
				tvv.ParentRenderWin().DNDNotCursor()
			case events.Hover:
				tvv.Open()
			}
		})
	*/
	if ftv.HasChildren() {
		if wb, ok := ftv.BranchPart(); ok {
			wb.OnClick(func(e events.Event) {
				ftv.ToggleClose()
			})
		}
	}
	if lbl, ok := ftv.LabelPart(); ok {
		// HiPri is needed to override label's native processing
		lbl.OnClick(func(e events.Event) {
			switch e.MouseButton() {
			case events.Left:
				ftv.SelectAction(e.SelectMode())
				e.SetHandled()
			case events.Right:
				ftv.This().(gi.Widget).ContextMenu()
				e.SetHandled()
			}
		})
		lbl.On(events.DoubleClick, func(e events.Event) {
			ftv.ToggleClose()
			e.SetHandled()
		})
	}
}

func (ftv *FileTreeView) KeyInput(kt events.Event) {
	if gi.KeyEventTrace {
		fmt.Printf("TreeView KeyInput: %v\n", ftv.Path())
	}
	kf := gi.KeyFun(kt.KeyChord())
	selMode := events.SelectModeBits(kt.Modifiers())

	if selMode == events.SelectOne {
		if ftv.SelectMode() {
			selMode = events.ExtendContinuous
		}
	}

	// first all the keys that work for inactive and active
	if !ftv.IsDisabled() && !kt.IsHandled() {
		switch kf {
		case gi.KeyFunDelete:
			ftv.DeleteFiles()
			kt.SetHandled()
			// todo: remove when gi issue 237 is resolved
		case gi.KeyFunBackspace:
			ftv.DeleteFiles()
			kt.SetHandled()
		case gi.KeyFunDuplicate:
			ftv.DuplicateFiles()
			kt.SetHandled()
		case gi.KeyFunInsert: // New File
			CallMethod(ftv, "NewFile", ftv.Sc)
			kt.SetHandled()
		case gi.KeyFunInsertAfter: // New Folder
			CallMethod(ftv, "NewFolder", ftv.Sc)
			kt.SetHandled()
		}
	}
	if !kt.IsHandled() {
		ftv.TreeView.KeyInput(kt)
	}
}

// ShowFileInfo calls ViewFile on selected files
func (ftv *FileTreeView) ShowFileInfo() {
	sels := ftv.SelectedViews()
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			StructViewDialog(ftv, DlgOpts{Title: "File Info", Inactive: true}, &fn.Info, nil)
		}
	}
}

// OpenFileDefault opens file with default app for that file type (os defined)
// runs open on Mac, xdg-open on Linux, and start on RenderWins
func (ftv *FileTreeView) OpenFileDefault() {
	sels := ftv.SelectedViews()
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			fn.OpenFileDefault()
		}
	}
}

// OpenFileWith opens file with user-specified command.
func (ftv *FileTreeView) OpenFileWith() {
	sels := ftv.SelectedViews()
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			CallMethod(fn, "OpenFileWith", ftv.Sc)
		}
	}
}

// DuplicateFiles calls DuplicateFile on any selected nodes
func (ftv *FileTreeView) DuplicateFiles() {
	sels := ftv.SelectedViews()
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			fn.DuplicateFile()
		}
	}
}

// DeleteFilesImpl does the actual deletion, no prompts
func (ftv *FileTreeView) DeleteFilesImpl() {
	sels := ftv.SelectedViews()
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn == nil {
			return
		}
		if fn.Info.IsDir() {
			openList := []string{}
			var fns []string
			fn.Info.FileNames(&fns)
			ft := fn.FRoot
			for _, filename := range fns {
				fn, ok := ft.FindFile(filename)
				if !ok {
					return
				}
				if fn.Buf != nil {
					openList = append(openList, filename)
				}
			}
			if len(openList) > 0 {
				for _, filename := range openList {
					fn, _ := ft.FindFile(filename)
					fn.CloseBuf()
				}
			}
			fn.DeleteFile()
		} else {
			fn.DeleteFile()
		}
	}
}

// DeleteFiles calls DeleteFile on any selected nodes. If any directory is selected
// all files and subdirectories are also deleted.
func (ftv *FileTreeView) DeleteFiles() {
	gi.ChoiceDialog(ftv, gi.DlgOpts{Title: "Delete Files?",
		Prompt: "Ok to delete file(s)?  This is not undoable and files are not moving to trash / recycle bin. If any selections are directories all files and subdirectories will also be deleted."},
		[]string{"Delete Files", "Cancel"}, func(dlg *gi.Dialog) {
			switch dlg.Data.(int) {
			case 0:
				ftv.DeleteFilesImpl()
			case 1:
				// do nothing
			}
		})
}

// RenameFiles calls RenameFile on any selected nodes
func (ftv *FileTreeView) RenameFiles() {
	sels := ftv.SelectedViews()
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			if fn.IsExternal() {
				continue
			}
			CallMethod(fn, "RenameFile", ftv.Sc)
		}
	}
}

// OpenDir opens given directory
func (ftv *FileTreeView) OpenDir() {
	sels := ftv.SelectedViews()
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			fn.OpenDir()
		}
	}
}

// OpenAll opens all directories under this one
func (ftv *FileTreeView) OpenAll() {
	fn := ftv.FileNode()
	if fn != nil {
		fn.OpenAll()
		ftv.TreeView.OpenAll() // view has to do it too
	}
}

// CloseAll closes all directories under this one
func (ftv *FileTreeView) CloseAll() {
	fn := ftv.FileNode()
	if fn != nil {
		fn.CloseAll()
		ftv.TreeView.CloseAll() // view has to do it too
	}
}

// SortBy determines how to sort the files in the directory -- default is alpha by name,
// optionally can be sorted by modification time.
func (ftv *FileTreeView) SortBy(modTime bool) {
	sels := ftv.SelectedViews()
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			fn.SortBy(modTime)
		}
	}
	// ftv.UpdateAllFiles()
	ftv.ReSync()
}

// NewFile makes a new file in given selected directory node
func (ftv *FileTreeView) NewFile(filename string, addToVcs bool) {
	sels := ftv.SelectedViews()
	sz := len(sels)
	if sz == 0 { // shouldn't happen
		return
	}
	sn := AsFileTreeView(sels[sz-1])
	fn := sn.FileNode()
	if fn != nil {
		fn.NewFile(filename, addToVcs)
	}
}

// NewFolder makes a new file in given selected directory node
func (ftv *FileTreeView) NewFolder(foldername string) {
	sels := ftv.SelectedViews()
	sz := len(sels)
	if sz == 0 { // shouldn't happen
		return
	}
	sn := AsFileTreeView(sels[sz-1])
	fn := sn.FileNode()
	if fn != nil {
		fn.NewFolder(foldername)
	}
}

// AddToVcs adds the file to version control system
func (ftv *FileTreeView) AddToVcs() {
	sels := ftv.SelectedViews()
	sz := len(sels)
	if sz == 0 { // shouldn't happen
		return
	}
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			fn.AddToVcs()
		}
	}
}

// DeleteFromVcs removes the file from version control system
func (ftv *FileTreeView) DeleteFromVcs() {
	sels := ftv.SelectedViews()
	sz := len(sels)
	if sz == 0 { // shouldn't happen
		return
	}
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			fn.DeleteFromVcs()
		}
	}
}

// CommitToVcs commits the file from version control system
func (ftv *FileTreeView) CommitToVcs() {
	sels := ftv.SelectedViews()
	sz := len(sels)
	if sz == 0 { // shouldn't happen
		return
	}
	sn := AsFileTreeView(sels[sz-1])
	fn := sn.FileNode()
	if fn != nil {
		CallMethod(fn, "CommitToVcs", ftv.Sc)
	}
}

// RevertVcs removes the file from version control system
func (ftv *FileTreeView) RevertVcs() {
	sels := ftv.SelectedViews()
	sz := len(sels)
	if sz == 0 { // shouldn't happen
		return
	}
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			fn.RevertVcs()
		}
	}
}

// DiffVcs shows the diffs between two versions of this file, given by the
// revision specifiers -- if empty, defaults to A = current HEAD, B = current WC file.
// -1, -2 etc also work as universal ways of specifying prior revisions.
// Diffs are shown in a DiffViewDialog.
func (ftv *FileTreeView) DiffVcs(rev_a, rev_b string) {
	sels := ftv.SelectedViews()
	sz := len(sels)
	if sz == 0 { // shouldn't happen
		return
	}
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			fn.DiffVcs(rev_a, rev_b)
		}
	}
}

// LogVcs shows the VCS log of commits for this file, optionally with a
// since date qualifier: If since is non-empty, it should be
// a date-like expression that the VCS will understand, such as
// 1/1/2020, yesterday, last year, etc.  SVN only understands a
// number as a maximum number of items to return.
// If allFiles is true, then the log will show revisions for all files, not just
// this one.
// Returns the Log and also shows it in a VCSLogView which supports further actions.
func (ftv *FileTreeView) LogVcs(allFiles bool, since string) {
	sels := ftv.SelectedViews()
	sz := len(sels)
	if sz == 0 { // shouldn't happen
		return
	}
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			fn.LogVcs(allFiles, since)
		}
	}
}

// BlameVcs shows the VCS blame report for this file, reporting for each line
// the revision and author of the last change.
func (ftv *FileTreeView) BlameVcs() {
	sels := ftv.SelectedViews()
	sz := len(sels)
	if sz == 0 { // shouldn't happen
		return
	}
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil {
			fn.BlameVcs()
		}
	}
}

// RemoveFromExterns removes file from list of external files
func (ftv *FileTreeView) RemoveFromExterns() {
	sels := ftv.SelectedViews()
	for i := len(sels) - 1; i >= 0; i-- {
		sn := AsFileTreeView(sels[i].This())
		fn := sn.FileNode()
		if fn != nil && fn.IsExternal() {
			fn.FRoot.RemoveExtFile(string(fn.FPath))
			fn.CloseBuf()
			fn.Delete(true)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
//   Clipboard

// MimeData adds mimedata for this node: a text/plain of the Path,
// text/plain of filename, and text/
func (ftv *FileTreeView) MimeData(md *mimedata.Mimes) {
	sroot := ftv.RootView.SrcNode
	fn := AsFileNode(ftv.SrcNode)
	path := string(fn.FPath)
	punq := fn.PathFrom(sroot)
	*md = append(*md, mimedata.NewTextData(punq))
	*md = append(*md, mimedata.NewTextData(path))
	if int(fn.Info.Size) < gi.Prefs.Params.BigFileSize {
		in, err := os.Open(path)
		if err != nil {
			log.Println(err)
			return
		}
		b, err := ioutil.ReadAll(in)
		if err != nil {
			log.Println(err)
			return
		}
		fd := &mimedata.Data{fn.Info.Mime, b}
		*md = append(*md, fd)
	} else {
		*md = append(*md, mimedata.NewTextData("File exceeds BigFileSize"))
	}
}

// Cut copies to clip.Board and deletes selected items
// satisfies gi.Clipper interface and can be overridden by subtypes
func (ftv *FileTreeView) Cut() {
	if ftv.IsRootOrField("Cut") {
		return
	}
	ftv.Copy(false)
	// todo: in the future, move files somewhere temporary, then use those temps for paste..
	gi.PromptDialog(ftv, gi.DlgOpts{Title: "Cut Not Supported", Prompt: "File names were copied to clipboard and can be pasted to copy elsewhere, but files are not deleted because contents of files are not placed on the clipboard and thus cannot be pasted as such.  Use Delete to delete files.", Ok: true, Cancel: false}, nil)
}

// Paste pastes clipboard at given node
// satisfies gi.Clipper interface and can be overridden by subtypes
func (ftv *FileTreeView) Paste() {
	md := ftv.EventMgr().ClipBoard().Read([]string{filecat.TextPlain})
	if md != nil {
		ftv.PasteMime(md)
	}
}

// Drop pops up a menu to determine what specifically to do with dropped items
// satisfies gi.DragNDropper interface and can be overridden by subtypes
func (ftv *FileTreeView) Drop(md mimedata.Mimes, mod events.DropMods) {
	ftv.PasteMime(md)
}

// DropExternal is not handled by base case but could be in derived
func (ftv *FileTreeView) DropExternal(md mimedata.Mimes, mod events.DropMods) {
	ftv.PasteMime(md)
}

// PasteCheckExisting checks for existing files in target node directory if
// that is non-nil (otherwise just uses absolute path), and returns list of existing
// and node for last one if exists.
func (ftv *FileTreeView) PasteCheckExisting(tfn *FileNode, md mimedata.Mimes) ([]string, *FileNode) {
	sroot := ftv.RootView.SrcNode
	tpath := ""
	if tfn != nil {
		tpath = string(tfn.FPath)
	}
	// intl := ftv.EventMgr.DNDIsInternalSrc() // todo
	intl := false
	nf := len(md)
	if intl {
		nf /= 3
	}
	var sfn *FileNode
	var existing []string
	for i := 0; i < nf; i++ {
		var d *mimedata.Data
		if intl {
			d = md[i*3+1]
			npath := string(md[i*3].Data)
			sfni, err := sroot.FindPathTry(npath)
			if err == nil {
				sfn = AsFileNode(sfni)
			}
		} else {
			d = md[i] // just a list
		}
		if d.Type != filecat.TextPlain {
			continue
		}
		path := string(d.Data)
		path = strings.TrimPrefix(path, "file://")
		if tfn != nil {
			_, fnm := filepath.Split(path)
			path = filepath.Join(tpath, fnm)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			existing = append(existing, path)
		}
	}
	return existing, sfn
}

// PasteCopyFiles copies files in given data into given target directory
func (ftv *FileTreeView) PasteCopyFiles(tdir *FileNode, md mimedata.Mimes) {
	sroot := ftv.RootView.SrcNode
	// intl := ftv.EventMgr.DNDIsInternalSrc()
	intl := true
	nf := len(md)
	if intl {
		nf /= 3
	}
	for i := 0; i < nf; i++ {
		var d *mimedata.Data
		mode := os.FileMode(0664)
		if intl {
			d = md[i*3+1]
			npath := string(md[i*3].Data)
			sfni, err := sroot.FindPathTry(npath)
			if err != nil {
				fmt.Println(err)
				continue
			}
			sfn := AsFileNode(sfni)
			mode = sfn.Info.Mode
		} else {
			d = md[i] // just a list
		}
		if d.Type != filecat.TextPlain {
			continue
		}
		path := string(d.Data)
		if strings.HasPrefix(path, "file://") {
			path = path[7:]
		}
		tdir.CopyFileToDir(path, mode)
	}
}

// PasteMimeCopyFilesCheck copies files into given directory node,
// first checking if any already exist -- if they exist, prompts.
func (ftv *FileTreeView) PasteMimeCopyFilesCheck(tdir *FileNode, md mimedata.Mimes) {
	// existing, _ := ftv.PasteCheckExisting(tdir, md)
	// if len(existing) > 0 {
	// 	gi.ChoiceDialog(ftv, gi.DlgOpts{Title: "File(s) Exist in Target Dir, Overwrite?",
	// 		Prompt: fmt.Sprintf("File(s): %v exist, do you want to overwrite?", existing)},
	// 		[]string{"No, Cancel", "Yes, Overwrite"}, func(dlg *gi.Dialog) {
	// 			switch dlg.Data.(int) {
	// 			case 0:
	// 				ftv.DropCancel()
	// 			case 1:
	// 				ftv.PasteCopyFiles(tdir, md)
	// 				ftv.DragNDropFinalizeDefMod()
	// 			}
	// 		})
	// } else {
	// 	ftv.PasteCopyFiles(tdir, md)
	// 	ftv.DragNDropFinalizeDefMod()
	// }
}

// PasteMime applies a paste / drop of mime data onto this node
// always does a copy of files into / onto target
func (ftv *FileTreeView) PasteMime(md mimedata.Mimes) {
	/*
		if len(md) == 0 {
			ftv.DropCancel()
			return
		}
		tfn := ftv.FileNode()
		if tfn == nil || tfn.IsExternal() {
			ftv.DropCancel()
			return
		}
		tupdt := ftv.RootView.UpdateStart()
		defer ftv.RootView.UpdateEnd(tupdt)
		tpath := string(tfn.FPath)
		isdir := tfn.IsDir()
		if isdir {
			ftv.PasteMimeCopyFilesCheck(tfn, md)
			return
		}
		if len(md) > 3 { // multiple files -- automatically goes into parent dir
			tdir := AsFileNode(tfn.Parent())
			ftv.PasteMimeCopyFilesCheck(tdir, md)
			return
		}
		// single file dropped onto a single target file
		srcpath := ""
		// intl := ftv.EventMgr.DNDIsInternalSrc() // todo
		intl := true
		if intl {
			srcpath = string(md[1].Data) // 1 has file path, 0 = ki path, 2 = file data
		} else {
			srcpath = string(md[0].Data) // just file path
		}
		fname := filepath.Base(srcpath)
		tdir := AsFileNode(tfn.Parent())
		existing, sfn := ftv.PasteCheckExisting(tdir, md)
		mode := os.FileMode(0664)
		if sfn != nil {
			mode = sfn.Info.Mode
		}
		if len(existing) == 1 && fname == tfn.Nm {
			gi.ChoiceDialog(nil, gi.DlgOpts{Title: "Overwrite?",
				Prompt: fmt.Sprintf("Overwrite target file: %s with source file of same name?, or diff (compare) two files, or cancel?", tfn.Nm)},
				[]string{"Overwrite Target", "Diff Files", "Cancel"}, func(dlg *gi.Dialog) {
					switch dlg.Data.(int) {
					case 0:
						CopyFile(tpath, srcpath, mode)
						ftv.DragNDropFinalizeDefMod()
					case 1:
						// DiffFiles(tpath, srcpath)
						ftv.DropCancel()
					case 2:
						ftv.DropCancel()
					}
				})
		} else if len(existing) > 0 {
			gi.ChoiceDialog(nil, gi.DlgOpts{Title: "Overwrite?",
				Prompt: fmt.Sprintf("Overwrite target file: %s with source file: %s, or overwrite existing file with same name as source file (%s), or diff (compare) files, or cancel?", tfn.Nm, fname, fname)},
				[]string{"Overwrite Target", "Overwrite Existing", "Diff to Target", "Diff to Existing", "Cancel"},
				func(dlg *gi.Dialog) {
					switch dlg.Data.(int) {
					case 0:
						CopyFile(tpath, srcpath, mode)
						ftv.DragNDropFinalizeDefMod()
					case 1:
						npath := filepath.Join(string(tdir.FPath), fname)
						CopyFile(npath, srcpath, mode)
						ftv.DragNDropFinalizeDefMod()
					case 2:
						// DiffFiles(tpath, srcpath)
						ftv.DropCancel()
					case 3:
						npath := filepath.Join(string(tdir.FPath), fname)
						_ = npath
						// DiffFiles(npath, srcpath)
						ftv.DropCancel()
					case 4:
						ftv.DropCancel()
					}
				})
		} else {
			gi.ChoiceDialog(nil, gi.DlgOpts{Title: "Overwrite?",
				Prompt: fmt.Sprintf("Overwrite target file: %s with source file: %s, or copy to: %s in current folder (which doesn't yet exist), or diff (compare) the two files, or cancel?", tfn.Nm, fname, fname)},
				[]string{"Overwrite Target", "Copy New File", "Diff Files", "Cancel"}, func(dlg *gi.Dialog) {
					switch dlg.Data.(int) {
					case 0:
						CopyFile(tpath, srcpath, mode)
						ftv.DragNDropFinalizeDefMod()
					case 1:
						tdir.CopyFileToDir(srcpath, mode) // does updating, vcs stuff
						ftv.DragNDropFinalizeDefMod()
					case 2:
						// DiffFiles(tpath, srcpath)
						ftv.DropCancel()
					case 3:
						ftv.DropCancel()
					}
				})
		}
	*/

}

// Dragged is called after target accepts the drop -- we just remove
// elements that were moved
// satisfies gi.DragNDropper interface and can be overridden by subtypes
func (ftv *FileTreeView) Dragged(de events.Event) {
	// fmt.Printf("ftv dragged: %v\n", ftv.Path())
	// if de.Mod != events.DropMove {
	// 	return
	// }
	sroot := AsFileTreeView(ftv.RootView.SrcNode)
	tfn := sroot.FileNode()
	if tfn == nil || tfn.IsExternal() {
		return
	}
	/*  // todo
	md := de.Data
	nf := len(md) / 3 // always internal
	for i := 0; i < nf; i++ {
		npath := string(md[i*3].Data)
		sfni, err := sroot.FindPathTry(npath)
		if err != nil {
			fmt.Println(err)
			continue
		}
		sfn := AsFileNode(sfni)
		if sfn == nil {
			continue
		}
		// fmt.Printf("dnd deleting: %v  path: %v\n", sfn.Path(), sfn.FPath)
		sfn.DeleteFile()
	}
	*/
}

// FileTreeInactiveExternFunc is an ActionUpdateFunc that inactivates action if node is external
var FileTreeInactiveExternFunc = ActionUpdateFunc(func(fni any, act *gi.Button) {
	ftv := AsFileTreeView(fni.(ki.Ki))
	fn := ftv.FileNode()
	if fn != nil {
		act.SetState(fn.IsExternal(), states.Disabled)
	}
})

// FileTreeActiveExternFunc is an ActionUpdateFunc that activates action if node is external
var FileTreeActiveExternFunc = ActionUpdateFunc(func(fni any, act *gi.Button) {
	ftv := AsFileTreeView(fni.(ki.Ki))
	fn := ftv.FileNode()
	if fn != nil {
		act.SetEnabledState(fn.IsExternal() && !fn.IsIrregular())
	}
})

// FileTreeInactiveDirFunc is an ActionUpdateFunc that inactivates action if node is a dir
var FileTreeInactiveDirFunc = ActionUpdateFunc(func(fni any, act *gi.Button) {
	ftv := AsFileTreeView(fni.(ki.Ki))
	fn := ftv.FileNode()
	if fn != nil {
		act.SetState(fn.IsDir() || fn.IsExternal(), states.Disabled)
	}
})

// FileTreeActiveDirFunc is an ActionUpdateFunc that activates action if node is a dir
var FileTreeActiveDirFunc = ActionUpdateFunc(func(fni any, act *gi.Button) {
	ftv := AsFileTreeView(fni.(ki.Ki))
	fn := ftv.FileNode()
	if fn != nil {
		act.SetEnabledState(fn.IsDir() && !fn.IsExternal())
	}
})

// FileTreeActiveNotInVcsFunc is an ActionUpdateFunc that inactivates action if node is not under version control
var FileTreeActiveNotInVcsFunc = ActionUpdateFunc(func(fni any, act *gi.Button) {
	ftv := AsFileTreeView(fni.(ki.Ki))
	fn := ftv.FileNode()
	if fn != nil {
		repo, _ := fn.Repo()
		if repo == nil || fn.IsDir() {
			act.SetEnabledState((false))
			return
		}
		act.SetEnabledState((fn.Info.Vcs == vci.Untracked))
	}
})

// FileTreeActiveInVcsFunc is an ActionUpdateFunc that activates action if node is under version control
var FileTreeActiveInVcsFunc = ActionUpdateFunc(func(fni any, act *gi.Button) {
	ftv := AsFileTreeView(fni.(ki.Ki))
	fn := ftv.FileNode()
	if fn != nil {
		repo, _ := fn.Repo()
		if repo == nil || fn.IsDir() {
			act.SetEnabledState((false))
			return
		}
		act.SetEnabledState((fn.Info.Vcs >= vci.Stored))
	}
})

// FileTreeActiveInVcsModifiedFunc is an ActionUpdateFunc that activates action if node is under version control
// and the file has been modified
var FileTreeActiveInVcsModifiedFunc = ActionUpdateFunc(func(fni any, act *gi.Button) {
	ftv := AsFileTreeView(fni.(ki.Ki))
	fn := ftv.FileNode()
	if fn != nil {
		repo, _ := fn.Repo()
		if repo == nil || fn.IsDir() {
			act.SetEnabledState((false))
			return
		}
		act.SetEnabledState((fn.Info.Vcs == vci.Modified || fn.Info.Vcs == vci.Added))
	}
})

// VcsGetRemoveLabelFunc gets the appropriate label for removing from version control
var VcsLabelFunc = LabelFunc(func(fni any, act *gi.Button) string {
	ftv := AsFileTreeView(fni.(ki.Ki))
	fn := ftv.FileNode()
	label := act.Text
	if fn != nil {
		repo, _ := fn.Repo()
		if repo != nil {
			label = strings.Replace(label, "Vcs", string(repo.Vcs()), 1)
		}
	}
	return label
})

var FileTreeViewProps = ki.Props{
	"CtxtMenuActive": ki.PropSlice{
		{"ShowFileInfo", ki.Props{
			"label": "File Info",
		}},
		{"OpenFileDefault", ki.Props{
			"label": "Open (w/default app)",
		}},
		{"sep-act", ki.BlankProp{}},
		{"DuplicateFiles", ki.Props{
			"label":    "Duplicate",
			"updtfunc": FileTreeInactiveDirFunc,
			"shortcut": gi.KeyFunDuplicate,
		}},
		{"DeleteFiles", ki.Props{
			"label":    "Delete",
			"desc":     "Ok to delete file(s)?  This is not undoable and is not moving to trash / recycle bin",
			"updtfunc": FileTreeInactiveExternFunc,
			"shortcut": gi.KeyFunDelete,
		}},
		{"RenameFiles", ki.Props{
			"label":    "Rename",
			"desc":     "Rename file to new file name",
			"updtfunc": FileTreeInactiveExternFunc,
		}},
		{"sep-open", ki.BlankProp{}},
		{"OpenAll", ki.Props{
			"updtfunc": FileTreeActiveDirFunc,
		}},
		{"CloseAll", ki.Props{
			"updtfunc": FileTreeActiveDirFunc,
		}},
		{"SortBy", ki.Props{
			"desc":     "Choose how to sort files in the directory -- default by Name, optionally can use Modification Time",
			"updtfunc": FileTreeActiveDirFunc,
			"Args": ki.PropSlice{
				{"Modification Time", ki.Props{}},
			},
		}},
		{"sep-new", ki.BlankProp{}},
		{"NewFile", ki.Props{
			"label":    "New File...",
			"desc":     "make a new file in this folder",
			"shortcut": gi.KeyFunInsert,
			"updtfunc": FileTreeActiveDirFunc,
			"Args": ki.PropSlice{
				{"File Name", ki.Props{
					"width": 60,
				}},
				{"Add To Version Control", ki.Props{}},
			},
		}},
		{"NewFolder", ki.Props{
			"label":    "New Folder...",
			"desc":     "make a new folder within this folder",
			"shortcut": gi.KeyFunInsertAfter,
			"updtfunc": FileTreeActiveDirFunc,
			"Args": ki.PropSlice{
				{"Folder Name", ki.Props{
					"width": 60,
				}},
			},
		}},
		{"sep-vcs", ki.BlankProp{}},
		{"AddToVcs", ki.Props{
			"desc":       "Add file to version control",
			"updtfunc":   FileTreeActiveNotInVcsFunc,
			"label-func": VcsLabelFunc,
		}},
		{"DeleteFromVcs", ki.Props{
			"desc":       "Delete file from version control",
			"updtfunc":   FileTreeActiveInVcsFunc,
			"label-func": VcsLabelFunc,
		}},
		{"CommitToVcs", ki.Props{
			"desc":       "Commit file to version control",
			"updtfunc":   FileTreeActiveInVcsModifiedFunc,
			"label-func": VcsLabelFunc,
		}},
		{"RevertVcs", ki.Props{
			"desc":       "Revert file to last commit",
			"updtfunc":   FileTreeActiveInVcsModifiedFunc,
			"label-func": VcsLabelFunc,
		}},
		{"sep-vcs-log", ki.BlankProp{}},
		{"DiffVcs", ki.Props{
			"desc":       "shows the diffs between two versions of this file, given by the revision specifiers -- if empty, defaults to A = current HEAD, B = current WC file.   -1, -2 etc also work as universal ways of specifying prior revisions.",
			"updtfunc":   FileTreeActiveInVcsFunc,
			"label-func": VcsLabelFunc,
			"Args": ki.PropSlice{
				{"Revision A", ki.Props{}},
				{"Revision B", ki.Props{}},
			},
		}},
		{"LogVcs", ki.Props{
			"desc":       "shows the VCS log of commits for this file, optionally with a since date qualifier: If since is non-empty, it should be a date-like expression that the VCS will understand, such as 1/1/2020, yesterday, last year, etc (SVN only supports a max number of entries).  If allFiles is true, then the log will show revisions for all files, not just this one.",
			"updtfunc":   FileTreeActiveInVcsFunc,
			"label-func": VcsLabelFunc,
			"Args": ki.PropSlice{
				{"All Files", ki.Props{}},
				{"Since Date", ki.Props{}},
			},
		}},
		{"BlameVcs", ki.Props{
			"desc":       "shows the VCS blame report for this file, reporting for each line the revision and author of the last change.",
			"updtfunc":   FileTreeActiveInVcsFunc,
			"label-func": VcsLabelFunc,
		}},
		{"sep-extrn", ki.BlankProp{}},
		{"RemoveFromExterns", ki.Props{
			"desc":       "Remove file from external files listt",
			"updtfunc":   FileTreeActiveExternFunc,
			"label-func": VcsLabelFunc,
		}},
	},
}

func (ft *FileTreeView) ApplyStyle(sc *gi.Scene) {
	fn := ft.FileNode()
	ft.Class = ""
	if fn != nil {
		if fn.IsDir() {
			if fn.HasChildren() {
				ft.Icon = ""
			} else {
				ft.Icon = icons.Folder
			}
			ft.AddClass("folder")
		} else {
			ft.Icon = fn.Info.Ic
			if ft.Icon.IsNil() {
				ft.Icon = icons.Blank
			}
			if fn.IsExec() {
				ft.AddClass("exec")
			}
			if fn.IsOpen() {
				ft.AddClass("open")
			}
			switch fn.Info.Vcs {
			case vci.Untracked:
				ft.AddClass("untracked")
			case vci.Stored:
				ft.AddClass("stored")
			case vci.Modified:
				ft.AddClass("modified")
			case vci.Added:
				ft.AddClass("added")
			case vci.Deleted:
				ft.AddClass("deleted")
			case vci.Conflicted:
				ft.AddClass("conflicted")
			case vci.Updated:
				ft.AddClass("updated")
			}
		}
		ft.StyleTreeView()
	}
}
