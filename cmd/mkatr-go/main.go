// SPDX-License-Identifier: GPL-2.0-only
// Command mkatr-go is a Go port of https://github.com/dmsc/mkatr.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/megalith/mkatrgo"
)

const version = "mkatr-go 1.0.0 (compatible with dmsc/mkatr)"

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] <output_atr> [+attributes] <file_1> [... <file_n>]\n", os.Args[0])
	fmt.Fprintln(flag.CommandLine.Output(), "Options:")
	fmt.Fprintln(flag.CommandLine.Output(), "  -b       next file is loaded at boot")
	fmt.Fprintln(flag.CommandLine.Output(), "  -x       exact sector count")
	fmt.Fprintln(flag.CommandLine.Output(), "  -s size  minimum image size in bytes")
	fmt.Fprintln(flag.CommandLine.Output(), "  -B page  bootloader page (4..239, default 7)")
	fmt.Fprintln(flag.CommandLine.Output(), "  -f fs    filesystem: sparta or eos")
	fmt.Fprintln(flag.CommandLine.Output(), "  -h       show help")
	fmt.Fprintln(flag.CommandLine.Output(), "  -v       show version")
	fmt.Fprintln(flag.CommandLine.Output(), "Attributes before a file: +p protected, +h hidden, +a archived")
	fmt.Fprintln(flag.CommandLine.Output(), "Explorer commands:")
	fmt.Fprintln(flag.CommandLine.Output(), "  catalog <image.atr>")
	fmt.Fprintln(flag.CommandLine.Output(), "  extract <image.atr> <entry> <output>")
	fmt.Fprintln(flag.CommandLine.Output(), "  put <image.atr> <directory> <host-path> [...]")
	fmt.Fprintln(flag.CommandLine.Output(), "  mkdir <image.atr> <directory>")
	fmt.Fprintln(flag.CommandLine.Output(), "  remove <image.atr> <entry>")
	fmt.Fprintln(flag.CommandLine.Output(), "  rename <image.atr> <old-entry> <new-entry>")
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "mkatr-go:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("missing output file name")
	}
	switch args[0] {
	case "catalog", "extract", "put", "mkdir", "remove", "rename":
		return runExplorerCommand(args)
	}
	builder := mkatrgo.NewBuilder()
	options := mkatrgo.Options{Filesystem: "sparta", BootPage: 7}
	var output string
	var attributes mkatrgo.Attributes
	bootNext, bootSelected := false, false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			usage()
			return nil
		case arg == "-v" || arg == "--version":
			fmt.Println(version)
			return nil
		case arg == "-b":
			if bootSelected || bootNext {
				return errors.New("can specify only one boot file")
			}
			bootNext = true
		case arg == "-x":
			options.ExactSize = true
		case arg == "-s" || arg == "-B" || arg == "-f":
			if i+1 >= len(args) {
				return fmt.Errorf("option %s needs an argument", arg)
			}
			i++
			if arg == "-f" {
				options.Filesystem = args[i]
				continue
			}
			value, err := strconv.ParseInt(args[i], 0, 32)
			if err != nil {
				return fmt.Errorf("invalid value for %s: %w", arg, err)
			}
			if arg == "-s" {
				options.MinimumSize = int(value)
			} else {
				options.BootPage = int(value)
			}
		case strings.HasPrefix(arg, "+"):
			for _, value := range arg[1:] {
				switch value {
				case '+':
				case 'p', 'P':
					attributes |= mkatrgo.AttributeProtected
				case 'h', 'H':
					attributes |= mkatrgo.AttributeHidden
				case 'a', 'A':
					attributes |= mkatrgo.AttributeArchived
				default:
					return fmt.Errorf("invalid attribute +%c", value)
				}
			}
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("invalid option %s", arg)
		case output == "" && !bootNext:
			output = arg
		default:
			if err := builder.AddCLIPath(arg, attributes, bootNext); err != nil {
				return err
			}
			if bootNext {
				bootSelected = true
			}
			bootNext, attributes = false, 0
		}
	}
	if output == "" {
		return errors.New("missing output file name")
	}
	if bootNext {
		return errors.New("-b must be followed by a file")
	}
	result, err := builder.Create(output, options)
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s: %d sectors x %d bytes (%d-byte ATR)\n", output, result.Sectors, result.SectorSize, result.Bytes)
	return nil
}

func runExplorerCommand(args []string) error {
	command := args[0]
	if len(args) < 2 {
		return fmt.Errorf("%s needs an ATR image", command)
	}
	image, err := mkatrgo.OpenImage(args[1])
	if err != nil {
		return err
	}
	switch command {
	case "catalog":
		if len(args) != 2 {
			return errors.New("usage: catalog <image.atr>")
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(image)
	case "extract":
		if len(args) != 4 {
			return errors.New("usage: extract <image.atr> <entry> <output>")
		}
		return image.Extract(args[2], args[3])
	case "put":
		if len(args) < 4 {
			return errors.New("usage: put <image.atr> <directory> <host-path> [...]")
		}
		if err = image.Put(args[3:], args[2]); err != nil {
			return err
		}
	case "mkdir":
		if len(args) != 3 {
			return errors.New("usage: mkdir <image.atr> <directory>")
		}
		if err = image.Mkdir(args[2]); err != nil {
			return err
		}
	case "remove":
		if len(args) != 3 {
			return errors.New("usage: remove <image.atr> <entry>")
		}
		if err = image.Remove(args[2]); err != nil {
			return err
		}
	case "rename":
		if len(args) != 4 {
			return errors.New("usage: rename <image.atr> <old-entry> <new-entry>")
		}
		if err = image.Rename(args[2], args[3]); err != nil {
			return err
		}
	}
	return image.Save(image.Path)
}
