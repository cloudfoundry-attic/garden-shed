// +build !linux

package rootfs_provider

import "os"

var Chown = os.Lchown
