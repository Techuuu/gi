// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package giv

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/c2h5oh/datasize"
	"goki.dev/icons"
	"goki.dev/ki/v2"
	"goki.dev/pi/v2/filecat"
	"goki.dev/vci/v2"
)

// FileInfo represents the information about a given file / directory,
// including icon, mimetype, etc
type FileInfo struct {

	// [tableview: no-header] icon for file
	Ic icons.Icon `tableview:"no-header" desc:"icon for file"`

	// name of the file, without any path
	Name string `width:"40" desc:"name of the file, without any path"`

	// size of the file in bytes
	Size FileSize `desc:"size of the file in bytes"`

	// type of file / directory -- shorter, more user-friendly version of mime type, based on category
	Kind string `width:"20" max-width:"20" desc:"type of file / directory -- shorter, more user-friendly version of mime type, based on category"`

	// [tableview: -] full official mime type of the contents
	Mime string `tableview:"-" desc:"full official mime type of the contents"`

	// [tableview: -] functional category of the file, based on mime data etc
	Cat filecat.Cat `tableview:"-" desc:"functional category of the file, based on mime data etc"`

	// [tableview: -] supported file type
	Sup filecat.Supported `tableview:"-" desc:"supported file type"`

	// file mode bits
	Mode os.FileMode `desc:"file mode bits"`

	// time that contents (only) were last modified
	ModTime FileTime `desc:"time that contents (only) were last modified"`

	// [tableview: -] version control system status, when enabled
	Vcs vci.FileStatus `tableview:"-" desc:"version control system status, when enabled"`

	// [tableview: -] full path to file, including name -- for file functions
	Path string `tableview:"-" desc:"full path to file, including name -- for file functions"`
}

func NewFileInfo(fname string) (*FileInfo, error) {
	fi := &FileInfo{}
	err := fi.InitFile(fname)
	return fi, err
}

// InitFile initializes a FileInfo based on a filename -- directly returns
// filepath.Abs or os.Stat error on the given file.  filename can be anything
// that works given current directory -- Path will contain the full
// filepath.Abs path, and Name will be just the filename.
func (fi *FileInfo) InitFile(fname string) error {
	path, err := filepath.Abs(fname)
	if err != nil {
		return err
	}
	fi.Path = path
	_, fi.Name = filepath.Split(path)
	return fi.Stat()
}

// Stat runs os.Stat on file, returns any error directly but otherwise updates
// file info, including mime type, which then drives Kind and Icon -- this is
// the main function to call to update state.
func (fi *FileInfo) Stat() error {
	info, err := os.Stat(fi.Path)
	if err != nil {
		return err
	}
	fi.Size = FileSize(info.Size())
	fi.Mode = info.Mode()
	fi.ModTime = FileTime(info.ModTime())
	if info.IsDir() {
		fi.Kind = "Folder"
		fi.Cat = filecat.Folder
		fi.Sup = filecat.AnyFolder
	} else {
		fi.Cat = filecat.Unknown
		fi.Sup = filecat.NoSupport
		fi.Kind = ""
		mtyp, _, err := filecat.MimeFromFile(fi.Path)
		if err == nil {
			fi.Mime = mtyp
			fi.Cat = filecat.CatFromMime(fi.Mime)
			fi.Sup = filecat.MimeSupported(fi.Mime)
			if fi.Cat != filecat.Unknown {
				fi.Kind = fi.Cat.String() + ": "
			}
			if fi.Sup != filecat.NoSupport {
				fi.Kind += fi.Sup.String()
			} else {
				fi.Kind += filecat.MimeSub(fi.Mime)
			}
		}
		if fi.Cat == filecat.Unknown {
			if fi.IsExec() {
				fi.Cat = filecat.Exe
			}
		}
	}
	icn, _ := fi.FindIcon()
	fi.Ic = icn
	return nil
}

// IsDir returns true if file is a directory (folder)
func (fi *FileInfo) IsDir() bool {
	return fi.Mode.IsDir()
}

// IsExec returns true if file is an executable file
func (fi *FileInfo) IsExec() bool {
	if fi.Mode&0111 != 0 {
		return true
	}
	ext := filepath.Ext(fi.Path)
	return ext == ".exe"
}

// IsSymLink returns true if file is a symbolic link
func (fi *FileInfo) IsSymlink() bool {
	return fi.Mode&os.ModeSymlink != 0
}

// IsHidden returns true if file name starts with . or _ which are typically hidden
func (fi *FileInfo) IsHidden() bool {
	return fi.Name == "" || fi.Name[0] == '.' || fi.Name[0] == '_'
}

//////////////////////////////////////////////////////////////////////////////
//    File ops

// Duplicate creates a copy of given file -- only works for regular files, not
// directories.
func (fi *FileInfo) Duplicate() (string, error) {
	if fi.IsDir() {
		err := fmt.Errorf("giv.Duplicate: cannot copy directory: %v", fi.Path)
		log.Println(err)
		return "", err
	}
	ext := filepath.Ext(fi.Path)
	noext := strings.TrimSuffix(fi.Path, ext)
	dst := noext + "_Copy" + ext
	cpcnt := 0
	for {
		if _, err := os.Stat(dst); !os.IsNotExist(err) {
			cpcnt++
			dst = noext + fmt.Sprintf("_Copy%d", cpcnt) + ext
		} else {
			break
		}
	}
	return dst, CopyFile(dst, fi.Path, fi.Mode)
}

// Delete deletes the file or if a directory the directory and all files and subdirectories
func (fi *FileInfo) Delete() error {
	if fi.IsDir() {
		path := fi.Path
		d, err := os.Open(path)
		if err != nil {
			return err
		}
		defer d.Close()
		names, err := d.Readdirnames(-1)
		if err != nil {
			return err
		}
		for _, name := range names {
			err = os.RemoveAll(filepath.Join(path, name))
			if err != nil {
				return err
			}
		}
	}
	// remove file or directory
	return os.Remove(fi.Path)
	// note: we should be deleted now!
}

// FileNames recursively adds fullpath filenames within the starting directory to the "names" slice.
// Directory names within the starting directory are not added.
func FileNames(d os.File, names *[]string) (err error) {
	nms, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, n := range nms {
		fp := filepath.Join(d.Name(), n)
		ffi, ferr := os.Stat(fp)
		if ferr != nil {
			return ferr
		}
		if ffi.IsDir() {
			dd, err := os.Open(fp)
			if err != nil {
				return err
			}
			defer dd.Close()
			FileNames(*dd, names)
		} else {
			*names = append(*names, fp)
		}
	}
	return nil
}

// FileNames returns a slice of file names from the starting directory and its subdirectories
func (fi *FileInfo) FileNames(names *[]string) (err error) {
	if !fi.IsDir() {
		err = errors.New("not a directory: FileNames returns a list of files within a directory")
		return err
	}
	path := fi.Path
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()

	err = FileNames(*d, names)
	return err
}

// RenamePath returns the proposed path or the new full path.
// Does not actually do the renaming -- see Rename method.
func (fi *FileInfo) RenamePath(path string) (newpath string, err error) {
	if path == "" {
		err = fmt.Errorf("giv.Rename: new name is empty")
		log.Println(err)
		return path, err
	}
	if path == fi.Path {
		return "", nil
	}
	ndir, np := filepath.Split(path)
	if ndir == "" {
		if np == fi.Name {
			return path, nil
		}
		dir, _ := filepath.Split(fi.Path)
		newpath = filepath.Join(dir, np)
	}
	return newpath, nil
}

// Rename renames (moves) this file to given new path name
// Updates the FileInfo setting to the new name, although it might
// be out of scope if it moved into a new path
func (fi *FileInfo) Rename(path string) (newpath string, err error) {
	orgpath := fi.Path
	newpath, err = fi.RenamePath(path)
	if err != nil {
		return
	}
	err = os.Rename(string(orgpath), newpath)
	if err == nil {
		fi.Path = newpath
		_, fi.Name = filepath.Split(newpath)
	}
	return
}

// FindIcon uses file info to find an appropriate icon for this file -- uses
// Kind string first to find a correspondingly-named icon, and then tries the
// extension.  Returns true on success.
func (fi *FileInfo) FindIcon() (icons.Icon, bool) {
	if fi.IsDir() {
		return "folder", true
	}
	if fi.Sup != filecat.NoSupport {
		snm := strings.ToLower(fi.Sup.String())
		if icn := icons.Icon(snm); icn.IsValid() {
			return icn, true
		}
		if icn := icons.Icon("file-" + snm); icn.IsValid() {
			return icn, true
		}
	}
	subt := strings.ToLower(filecat.MimeSub(fi.Mime))
	if subt != "" {
		if icn := icons.Icon(subt); icn.IsValid() {
			return icn, true
		}
	}
	if fi.Cat != filecat.Unknown {
		cat := strings.ToLower(fi.Cat.String())
		if icn := icons.Icon(cat); icn.IsValid() {
			return icn, true
		}
		if icn := icons.Icon("file-" + cat); icn.IsValid() {
			return icn, true
		}
	}
	ext := filepath.Ext(fi.Name)
	if ext != "" {
		if icn := icons.Icon(ext[1:]); icn.IsValid() {
			return icn, true
		}
	}

	icn := icons.None
	return icn, false
}

var FileInfoProps = ki.Props{
	"CtxtMenu": ki.PropSlice{
		{"Duplicate", ki.Props{
			"desc":    "Duplicate this file or folder",
			"confirm": true,
		}},
		{"Delete", ki.Props{
			"desc":    "Ok to delete this file or folder?  This is not undoable and is not moving to trash / recycle bin",
			"confirm": true,
		}},
		{"Rename", ki.Props{
			"desc": "Rename file to new file name",
			"Args": ki.PropSlice{
				{"New Name", ki.Props{
					"default-field": "Name",
				}},
			},
		}},
	},
}

//////////////////////////////////////////////////////////////////////////////
//    FileTime, FileSize

// Note: can get all the detailed birth, access, change times from this package
// 	"github.com/djherbis/times"

// FileTime provides a default String format for file modification times, and
// other useful methods -- will plug into ValueView with date / time editor.
type FileTime time.Time

// Int satisfies the ints.Inter interface for sorting etc
func (ft FileTime) Int() int64 {
	return (time.Time(ft)).Unix()
}

// FromInt satisfies the ints.Inter interface
func (ft *FileTime) FromInt(val int64) {
	*ft = FileTime(time.Unix(val, 0))
}

func (ft FileTime) String() string {
	return (time.Time)(ft).Format("Mon Jan  2 15:04:05 MST 2006")
}

func (ft FileTime) MarshalBinary() ([]byte, error) {
	return time.Time(ft).MarshalBinary()
}

func (ft FileTime) MarshalJSON() ([]byte, error) {
	return time.Time(ft).MarshalJSON()
}

func (ft FileTime) MarshalText() ([]byte, error) {
	return time.Time(ft).MarshalText()
}

func (ft *FileTime) UnmarshalBinary(data []byte) error {
	return (*time.Time)(ft).UnmarshalBinary(data)
}

func (ft *FileTime) UnmarshalJSON(data []byte) error {
	return (*time.Time)(ft).UnmarshalJSON(data)
}

func (ft *FileTime) UnmarshalText(data []byte) error {
	return (*time.Time)(ft).UnmarshalText(data)
}

type FileSize datasize.ByteSize

// Int satisfies the kit.Inter interface for sorting etc
func (fs FileSize) Int() int64 {
	return int64(fs) // note: is actually uint64
}

// FromInt satisfies the ints.Inter interface
func (fs *FileSize) FromInt(val int64) {
	*fs = FileSize(val)
}

func (fs FileSize) String() string {
	return (datasize.ByteSize)(fs).HumanReadable()
}

//////////////////////////////////////////////////////////////////////////////
//    CopyFile

// here's all the discussion about why CopyFile is not in std lib:
// https://old.reddit.com/r/golang/comments/3lfqoh/why_golang_does_not_provide_a_copy_file_func/
// https://github.com/golang/go/issues/8868

// CopyFile copies the contents from src to dst atomically.
// If dst does not exist, CopyFile creates it with permissions perm.
// If the copy fails, CopyFile aborts and dst is preserved.
func CopyFile(dst, src string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := ioutil.TempFile(filepath.Dir(dst), "")
	if err != nil {
		return err
	}
	_, err = io.Copy(tmp, in)
	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err = tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err = os.Chmod(tmp.Name(), perm); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), dst)
}
