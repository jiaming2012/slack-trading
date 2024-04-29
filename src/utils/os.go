package utils

import (
	"bufio"
	"os"
	"strings"
)

func ReadLineFromStdin(output *string) error {
	reader := bufio.NewReader(os.Stdin)
	o, err := reader.ReadString('\n')
	if err != nil {
		*output = ""
		return err
	}

	o = strings.TrimRight(o, "\n")
	*output = o
	return nil
}
