package main

import (
	"fmt"
	"os"
	"time"
)

type fileman struct {
	working_directory string
}

func (f *fileman) getFileWithTimestamp(prepend string, middle string, extension string) (*os.File, error) {
	formatted := time.Now().Format(time.RFC3339)
	cwd := f.working_directory

	if len(f.working_directory) < 1 {
		var err_cwd error

		cwd, err_cwd = os.Getwd()
		if err_cwd != nil {
			cwd = "."
			fmt.Println(err_cwd)
		}
	}

	var filePath = ""
	if len(middle) > 0 {
		filePath = fmt.Sprintf("%s/%s_%s_%s.%s", cwd, prepend, middle, formatted, extension)
	} else {
		filePath = fmt.Sprintf("%s/%s_%s.%s", cwd, prepend, formatted, extension)
	}
	return os.Create(filePath)
}
