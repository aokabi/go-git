package filemode

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ModeSuite struct {
	suite.Suite
}

func TestModeSuite(t *testing.T) {
	suite.Run(t, new(ModeSuite))
}

func (s *ModeSuite) TestNew() {
	for _, test := range [...]struct {
		input    string
		expected FileMode
	}{
		// these are the ones used in the packfile codification
		// of the tree entries
		{input: "40000", expected: Dir},
		{input: "100644", expected: Regular},
		{input: "100664", expected: Deprecated},
		{input: "100755", expected: Executable},
		{input: "120000", expected: Symlink},
		{input: "160000", expected: Submodule},
		// these are are not used by standard git to codify modes in
		// packfiles, but they often appear when parsing some git
		// outputs ("git diff-tree", for instance).
		{input: "000000", expected: Empty},
		{input: "040000", expected: Dir},
		// these are valid inputs, but probably means there is a bug
		// somewhere.
		{input: "0", expected: Empty},
		{input: "42", expected: FileMode(042)},
		{input: "00000000000100644", expected: Regular},
	} {
		comment := fmt.Sprintf("input = %q", test.input)
		obtained, err := New(test.input)
		s.Equal(test.expected, obtained, comment)
		s.NoError(err, comment)
	}
}

func (s *ModeSuite) TestNewErrors() {
	for _, input := range [...]string{
		"0x81a4",     // Regular in hex
		"-rw-r--r--", // Regular in default UNIX representation
		"",
		"-42",
		"9",  // this is no octal
		"09", // looks like octal, but it is not
		"mode",
		"-100644",
		"+100644",
	} {
		comment := fmt.Sprintf("input = %q", input)
		obtained, err := New(input)
		s.Equal(Empty, obtained, comment)
		s.NotNil(err, comment)
	}
}

// fixtures for testing NewModeFromOSFileMode
type fixture struct {
	input    os.FileMode
	expected FileMode
	err      string // error regexp, empty string for nil error
}

func (f fixture) test(s *ModeSuite) {
	obtained, err := NewFromOSFileMode(f.input)
	comment := fmt.Sprintf("input = %s (%07o)", f.input, uint32(f.input))
	s.Equal(f.expected, obtained, comment)
	if f.err != "" {
		s.ErrorContains(err, f.err, comment)
	} else {
		s.NoError(err, comment)
	}
}

func (s *ModeSuite) TestNewFromOsFileModeSimplePerms() {
	for _, f := range [...]fixture{
		{os.FileMode(0755) | os.ModeDir, Dir, ""},         // drwxr-xr-x
		{os.FileMode(0700) | os.ModeDir, Dir, ""},         // drwx------
		{os.FileMode(0500) | os.ModeDir, Dir, ""},         // dr-x------
		{os.FileMode(0644), Regular, ""},                  // -rw-r--r--
		{os.FileMode(0660), Regular, ""},                  // -rw-rw----
		{os.FileMode(0640), Regular, ""},                  // -rw-r-----
		{os.FileMode(0600), Regular, ""},                  // -rw-------
		{os.FileMode(0400), Regular, ""},                  // -r--------
		{os.FileMode(0000), Regular, ""},                  // ----------
		{os.FileMode(0755), Executable, ""},               // -rwxr-xr-x
		{os.FileMode(0700), Executable, ""},               // -rwx------
		{os.FileMode(0500), Executable, ""},               // -r-x------
		{os.FileMode(0744), Executable, ""},               // -rwxr--r--
		{os.FileMode(0540), Executable, ""},               // -r-xr-----
		{os.FileMode(0550), Executable, ""},               // -r-xr-x---
		{os.FileMode(0777) | os.ModeSymlink, Symlink, ""}, // Lrwxrwxrwx
	} {
		f.test(s)
	}
}

func (s *ModeSuite) TestNewFromOsFileModeAppend() {
	// append files are just regular files
	fixture{
		input:    os.FileMode(0644) | os.ModeAppend, // arw-r--r--
		expected: Regular, err: "",
	}.test(s)
}

func (s *ModeSuite) TestNewFromOsFileModeExclusive() {
	// exclusive files are just regular or executable files
	fixture{
		input:    os.FileMode(0644) | os.ModeExclusive, // lrw-r--r--
		expected: Regular, err: "",
	}.test(s)

	fixture{
		input:    os.FileMode(0755) | os.ModeExclusive, // lrwxr-xr-x
		expected: Executable, err: "",
	}.test(s)
}

func (s *ModeSuite) TestNewFromOsFileModeTemporary() {
	// temporary files are ignored
	fixture{
		input:    os.FileMode(0644) | os.ModeTemporary, // Trw-r--r--
		expected: Empty, err: "no equivalent",
	}.test(s)

	fixture{
		input:    os.FileMode(0755) | os.ModeTemporary, // Trwxr-xr-x
		expected: Empty, err: "no equivalent",
	}.test(s)
}

func (s *ModeSuite) TestNewFromOsFileModeDevice() {
	// device files has no git equivalent
	fixture{
		input:    os.FileMode(0644) | os.ModeDevice, // Drw-r--r--
		expected: Empty, err: "no equivalent",
	}.test(s)
}

func (s *ModeSuite) TestNewFromOsFileNamedPipe() {
	// named pipes files has not git equivalent
	fixture{
		input:    os.FileMode(0644) | os.ModeNamedPipe, // prw-r--r--
		expected: Empty, err: "no equivalent",
	}.test(s)
}

func (s *ModeSuite) TestNewFromOsFileModeSocket() {
	// sockets has no git equivalent
	fixture{
		input:    os.FileMode(0644) | os.ModeSocket, // Srw-r--r--
		expected: Empty, err: "no equivalent",
	}.test(s)
}

func (s *ModeSuite) TestNewFromOsFileModeSetuid() {
	// Setuid are just executables
	fixture{
		input:    os.FileMode(0755) | os.ModeSetuid, // urwxr-xr-x
		expected: Executable, err: "",
	}.test(s)
}

func (s *ModeSuite) TestNewFromOsFileModeSetgid() {
	// Setguid are regular or executables, depending on the owner perms
	fixture{
		input:    os.FileMode(0644) | os.ModeSetgid, // grw-r--r--
		expected: Regular, err: "",
	}.test(s)

	fixture{
		input:    os.FileMode(0755) | os.ModeSetgid, // grwxr-xr-x
		expected: Executable, err: "",
	}.test(s)
}

func (s *ModeSuite) TestNewFromOsFileModeCharDevice() {
	// char devices has no git equivalent
	fixture{
		input:    os.FileMode(0644) | os.ModeCharDevice, // crw-r--r--
		expected: Empty, err: "no equivalent",
	}.test(s)
}

func (s *ModeSuite) TestNewFromOsFileModeSticky() {
	// dirs with the sticky bit are just dirs
	fixture{
		input:    os.FileMode(0755) | os.ModeDir | os.ModeSticky, // dtrwxr-xr-x
		expected: Dir, err: "",
	}.test(s)
}

func (s *ModeSuite) TestByte() {
	for _, test := range [...]struct {
		input    FileMode
		expected []byte
	}{
		{FileMode(0), []byte{0x00, 0x00, 0x00, 0x00}},
		{FileMode(1), []byte{0x01, 0x00, 0x00, 0x00}},
		{FileMode(15), []byte{0x0f, 0x00, 0x00, 0x00}},
		{FileMode(16), []byte{0x10, 0x00, 0x00, 0x00}},
		{FileMode(255), []byte{0xff, 0x00, 0x00, 0x00}},
		{FileMode(256), []byte{0x00, 0x01, 0x00, 0x00}},
		{Empty, []byte{0x00, 0x00, 0x00, 0x00}},
		{Dir, []byte{0x00, 0x40, 0x00, 0x00}},
		{Regular, []byte{0xa4, 0x81, 0x00, 0x00}},
		{Deprecated, []byte{0xb4, 0x81, 0x00, 0x00}},
		{Executable, []byte{0xed, 0x81, 0x00, 0x00}},
		{Symlink, []byte{0x00, 0xa0, 0x00, 0x00}},
		{Submodule, []byte{0x00, 0xe0, 0x00, 0x00}},
	} {
		s.Equal(test.expected, test.input.Bytes(),
			fmt.Sprintf("input = %s", test.input))
	}
}

func (s *ModeSuite) TestIsMalformed() {
	for _, test := range [...]struct {
		mode     FileMode
		expected bool
	}{
		{Empty, true},
		{Dir, false},
		{Regular, false},
		{Deprecated, false},
		{Executable, false},
		{Symlink, false},
		{Submodule, false},
		{FileMode(01), true},
		{FileMode(010), true},
		{FileMode(0100), true},
		{FileMode(01000), true},
		{FileMode(010000), true},
		{FileMode(0100000), true},
	} {
		s.Equal(test.expected, test.mode.IsMalformed())
	}
}

func (s *ModeSuite) TestString() {
	for _, test := range [...]struct {
		mode     FileMode
		expected string
	}{
		{Empty, "0000000"},
		{Dir, "0040000"},
		{Regular, "0100644"},
		{Deprecated, "0100664"},
		{Executable, "0100755"},
		{Symlink, "0120000"},
		{Submodule, "0160000"},
		{FileMode(01), "0000001"},
		{FileMode(010), "0000010"},
		{FileMode(0100), "0000100"},
		{FileMode(01000), "0001000"},
		{FileMode(010000), "0010000"},
		{FileMode(0100000), "0100000"},
	} {
		s.Equal(test.expected, test.mode.String())
	}
}

func (s *ModeSuite) TestIsRegular() {
	for _, test := range [...]struct {
		mode     FileMode
		expected bool
	}{
		{Empty, false},
		{Dir, false},
		{Regular, true},
		{Deprecated, true},
		{Executable, false},
		{Symlink, false},
		{Submodule, false},
		{FileMode(01), false},
		{FileMode(010), false},
		{FileMode(0100), false},
		{FileMode(01000), false},
		{FileMode(010000), false},
		{FileMode(0100000), false},
	} {
		s.Equal(test.expected, test.mode.IsRegular())
	}
}

func (s *ModeSuite) TestIsFile() {
	for _, test := range [...]struct {
		mode     FileMode
		expected bool
	}{
		{Empty, false},
		{Dir, false},
		{Regular, true},
		{Deprecated, true},
		{Executable, true},
		{Symlink, true},
		{Submodule, false},
		{FileMode(01), false},
		{FileMode(010), false},
		{FileMode(0100), false},
		{FileMode(01000), false},
		{FileMode(010000), false},
		{FileMode(0100000), false},
	} {
		s.Equal(test.expected, test.mode.IsFile())
	}
}

func (s *ModeSuite) TestToOSFileMode() {
	for _, test := range [...]struct {
		input     FileMode
		expected  os.FileMode
		errRegExp string // empty string for nil error
	}{
		{Empty, os.FileMode(0), "malformed"},
		{Dir, os.ModePerm | os.ModeDir, ""},
		{Regular, os.FileMode(0644), ""},
		{Deprecated, os.FileMode(0644), ""},
		{Executable, os.FileMode(0755), ""},
		{Symlink, os.ModePerm | os.ModeSymlink, ""},
		{Submodule, os.ModePerm | os.ModeDir, ""},
		{FileMode(01), os.FileMode(0), "malformed"},
		{FileMode(010), os.FileMode(0), "malformed"},
		{FileMode(0100), os.FileMode(0), "malformed"},
		{FileMode(01000), os.FileMode(0), "malformed"},
		{FileMode(010000), os.FileMode(0), "malformed"},
		{FileMode(0100000), os.FileMode(0), "malformed"},
	} {
		obtained, err := test.input.ToOSFileMode()
		comment := fmt.Sprintf("input = %s", test.input)
		if test.errRegExp != "" {
			s.Equal(os.FileMode(0), obtained, comment)
			s.ErrorContains(err, test.errRegExp, comment)
		} else {
			s.Equal(test.expected, obtained, comment)
			s.NoError(err, comment)
		}
	}
}
