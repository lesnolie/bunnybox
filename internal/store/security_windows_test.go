//go:build windows

package store

import (
	"golang.org/x/sys/windows"
	"path/filepath"
	"testing"
)

func TestNewDataDirectoryIsPrivate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	if e := SecureDirectory(root); e != nil {
		t.Fatal(e)
	}
	sd, e := windows.GetNamedSecurityInfo(root, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if e != nil {
		t.Fatal(e)
	}
	acl, _, e := sd.DACL()
	if e != nil {
		t.Fatal(e)
	}
	if acl == nil || acl.AceCount != 3 {
		t.Fatalf("want user/SYSTEM/Administrators ACL; got %+v", acl)
	}
	owner, _, e := sd.Owner()
	if e != nil {
		t.Fatal(e)
	}
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		t.Fatal(e)
	}
	if !windows.EqualSid(owner, user.User.Sid) {
		t.Fatal("unexpected directory owner")
	}
	control, _, e := sd.Control()
	if e != nil {
		t.Fatal(e)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("private directory inherits parent ACL")
	}
}
