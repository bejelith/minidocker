package mount

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// readRootFSMount discovers the device mounted to / and returns it's major and minor
// error is returned if no device is found.
func ReadRootFSMount(reader *bufio.Reader) (uint, uint, error) {
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		tokens := strings.Split(line, " ")
		// 6 mandatory field as Kernel specification
		if len(tokens) < 6 {
			return 0, 0, fmt.Errorf(
				"the kernel may be wrong, found %d columns in mountinfo",
				len(tokens))
		}
		if tokens[3] == tokens[4] && tokens[3] == "/" {
			majorMinor := strings.Split(tokens[2], ":")
			if len(majorMinor) != 2 {
				return 0, 0, fmt.Errorf(
					"the kernel may be wrong, found %d columns as maj:min:%s",
					len(tokens),
					line)
			}
			major, _ := strconv.Atoi(majorMinor[0])
			minor, _ := strconv.Atoi(majorMinor[1])
			return uint(major), uint(minor), nil
		}
	}
	return 0, 0, fmt.Errorf("no root device found")
}
