// SPDX-License-Identifier: MIT OR Unlicense

package file

import (
	"os"
)

// IsHidden Returns true if file is hidden
func IsHidden(file os.DirEntry, directory string) (bool, error) {
	return file.Name()[0:1] == ".", nil
}
