// SPDX-License-Identifier: GPL-2.0-only

package mkatrgo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ImageEntry is one file or directory stored in an ATR image.
type ImageEntry struct {
	Path       string     `json:"path"`
	Name       string     `json:"name"`
	Size       int        `json:"size"`
	Directory  bool       `json:"directory"`
	Attributes Attributes `json:"attributes"`
	Boot       bool       `json:"boot"`
	Modified   time.Time  `json:"modified"`
	data       []byte
}

// Image is an editable SDFS or EOSFS ATR image.
type Image struct {
	Path       string       `json:"path"`
	Filesystem string       `json:"filesystem"`
	SectorSize int          `json:"sectorSize"`
	Sectors    int          `json:"sectors"`
	Bytes      int          `json:"bytes"`
	FreeBytes  int          `json:"freeBytes"`
	Entries    []ImageEntry `json:"entries"`
	bootPage   int
}

// OpenImage reads and validates an ATR image and its SDFS/EOSFS directory tree.
func OpenImage(path string) (*Image, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	image, err := ParseImage(raw)
	if err != nil {
		return nil, fmt.Errorf("read ATR %q: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	image.Path = path
	return image, nil
}

// ParseImage decodes an ATR container and its SDFS/EOSFS volume.
func ParseImage(raw []byte) (*Image, error) {
	if len(raw) < 16 || raw[0] != 0x96 || raw[1] != 0x02 {
		return nil, errors.New("not an ATR image")
	}
	payloadSize := (int(raw[2]) | int(raw[3])<<8 | int(raw[6])<<16) * 16
	if payloadSize <= 0 || len(raw) < 16+payloadSize {
		return nil, errors.New("truncated ATR payload")
	}
	sectorSize := int(binary.LittleEndian.Uint16(raw[4:6]))
	if sectorSize != 128 && sectorSize != 256 {
		return nil, fmt.Errorf("unsupported ATR sector size %d", sectorSize)
	}
	sectors := payloadSize / sectorSize
	if sectorSize == 256 {
		if payloadSize < 384 || (payloadSize-384)%256 != 0 {
			return nil, errors.New("invalid mixed-density ATR length")
		}
		sectors = 3 + (payloadSize-384)/256
	}
	if sectors < 6 || sectors > 65535 {
		return nil, fmt.Errorf("invalid ATR sector count %d", sectors)
	}
	logical := make([]byte, sectors*sectorSize)
	payload := raw[16 : 16+payloadSize]
	if sectorSize == 128 {
		copy(logical, payload)
	} else {
		for index := 0; index < 3; index++ {
			copy(logical[index*256:index*256+128], payload[index*128:(index+1)*128])
		}
		copy(logical[3*256:], payload[384:])
	}
	reader := imageReader{data: logical, sectorSize: sectorSize, sectors: sectors}
	if len(logical) < 42 {
		return nil, errors.New("missing filesystem header")
	}
	revision := logical[32]
	eos := revision == 0x45 || revision == 0x46
	if !eos && revision != 0x20 {
		return nil, fmt.Errorf("unsupported SDFS/EOSFS revision $%02X", revision)
	}
	reader.eos = eos
	reader.bootMap = int(binary.LittleEndian.Uint16(logical[40:42]))
	mainMap := int(binary.LittleEndian.Uint16(logical[9:11]))
	if mainMap < 1 || mainMap > sectors {
		return nil, errors.New("invalid MAIN directory map")
	}
	entries, err := reader.readDirectory(mainMap, "", map[int]bool{})
	if err != nil {
		return nil, err
	}
	filesystem := "sparta"
	if eos {
		filesystem = "eos"
	}
	freeSectors := int(binary.LittleEndian.Uint16(logical[13:15]))
	bootPage := int(logical[3]) - 1
	if sectorSize == 256 {
		bootPage = int(logical[3]) - 2
	}
	if bootPage < 4 || bootPage > 239 {
		bootPage = 7
	}
	return &Image{Filesystem: filesystem, SectorSize: sectorSize, Sectors: sectors, Bytes: 16 + payloadSize, FreeBytes: freeSectors * sectorSize, Entries: entries, bootPage: bootPage}, nil
}

type imageReader struct {
	data                []byte
	sectorSize, sectors int
	eos                 bool
	bootMap             int
}

func (r *imageReader) sector(number int) ([]byte, error) {
	if number < 1 || number > r.sectors {
		return nil, fmt.Errorf("sector %d is outside the image", number)
	}
	start := (number - 1) * r.sectorSize
	return r.data[start : start+r.sectorSize], nil
}

func (r *imageReader) readData(mapSector, size int) ([]byte, error) {
	if size < 0 || size > maxFileBytes {
		return nil, fmt.Errorf("invalid entry size %d", size)
	}
	if size == 0 {
		return []byte{}, nil
	}
	if mapSector == 0 {
		return nil, errors.New("nonempty entry has no sector map")
	}
	result := make([]byte, 0, size)
	seen := make(map[int]bool)
	for mapSector != 0 && len(result) < size {
		if seen[mapSector] {
			return nil, errors.New("sector map loop")
		}
		seen[mapSector] = true
		block, err := r.sector(mapSector)
		if err != nil {
			return nil, err
		}
		if r.eos {
			for offset := 4; offset+4 <= len(block) && len(result) < size; offset += 4 {
				start := int(binary.LittleEndian.Uint16(block[offset : offset+2]))
				count := int(binary.LittleEndian.Uint16(block[offset+2 : offset+4]))
				if count == 0 {
					break
				}
				for index := 0; index < count && len(result) < size; index++ {
					if start == 0 {
						result = appendSectorBytes(result, make([]byte, r.sectorSize), size)
						continue
					}
					data, err := r.sector(start + index)
					if err != nil {
						return nil, err
					}
					result = appendSectorBytes(result, data, size)
				}
			}
		} else {
			for offset := 4; offset+2 <= len(block) && len(result) < size; offset += 2 {
				dataSector := int(binary.LittleEndian.Uint16(block[offset : offset+2]))
				if dataSector == 0 {
					break
				}
				data, err := r.sector(dataSector)
				if err != nil {
					return nil, err
				}
				result = appendSectorBytes(result, data, size)
			}
		}
		mapSector = int(binary.LittleEndian.Uint16(block[0:2]))
	}
	if len(result) != size {
		return nil, fmt.Errorf("entry data is truncated: got %d of %d bytes", len(result), size)
	}
	return result, nil
}

func appendSectorBytes(output, sector []byte, size int) []byte {
	remaining := size - len(output)
	if remaining < len(sector) {
		sector = sector[:remaining]
	}
	return append(output, sector...)
}

func (r *imageReader) readDirectory(mapSector int, parent string, ancestors map[int]bool) ([]ImageEntry, error) {
	if ancestors[mapSector] {
		return nil, errors.New("directory loop")
	}
	ancestors = cloneSet(ancestors)
	ancestors[mapSector] = true
	header, err := r.readData(mapSector, 23)
	if err != nil {
		return nil, err
	}
	size := int(header[3]) | int(header[4])<<8 | int(header[5])<<16
	if size < 23 || size > maxDirectoryBytes || size%23 != 0 {
		return nil, fmt.Errorf("invalid directory size %d", size)
	}
	data, err := r.readData(mapSector, size)
	if err != nil {
		return nil, err
	}
	entries := make([]ImageEntry, 0, size/23-1)
	for offset := 23; offset+23 <= len(data); offset += 23 {
		row := data[offset : offset+23]
		if row[0]&0x08 == 0 || row[0]&0x10 != 0 {
			continue
		}
		name := displayNameBytes(row[6:17])
		if name == "" {
			continue
		}
		path := name
		if parent != "" {
			path = parent + "/" + name
		}
		childMap := int(binary.LittleEndian.Uint16(row[1:3]))
		childSize := int(row[3]) | int(row[4])<<8 | int(row[5])<<16
		directory := row[0]&0x20 != 0
		entry := ImageEntry{Path: path, Name: name, Size: childSize, Directory: directory, Attributes: Attributes(row[0] & 0x07), Boot: !r.eos && childMap == r.bootMap, Modified: decodeTimestamp(row[17:23])}
		if directory {
			children, err := r.readDirectory(childMap, path, ancestors)
			if err != nil {
				return nil, fmt.Errorf("read directory %s: %w", path, err)
			}
			entries = append(entries, entry)
			entries = append(entries, children...)
		} else {
			entry.data, err = r.readData(childMap, childSize)
			if err != nil {
				return nil, fmt.Errorf("read file %s: %w", path, err)
			}
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func cloneSet(input map[int]bool) map[int]bool {
	output := make(map[int]bool, len(input)+1)
	for key, value := range input {
		output[key] = value
	}
	return output
}

func displayNameBytes(value []byte) string {
	base := strings.TrimRight(string(value[:8]), " \x00")
	ext := strings.TrimRight(string(value[8:11]), " \x00")
	if ext != "" {
		return base + "." + ext
	}
	return base
}

func decodeTimestamp(value []byte) time.Time {
	if len(value) < 6 || value[0] == 0 || value[1] == 0 {
		return time.Time{}
	}
	year := 1900 + int(value[2])
	if value[2] < 80 {
		year += 100
	}
	result := time.Date(year, time.Month(value[1]), int(value[0]), int(value[3]), int(value[4]), int(value[5]), 0, time.Local)
	if result.Day() != int(value[0]) || result.Month() != time.Month(value[1]) {
		return time.Time{}
	}
	return result
}

// Save rewrites the image atomically while preserving its geometry and filesystem.
func (image *Image) Save(path string) error {
	builder := NewBuilder()
	entries := append([]ImageEntry(nil), image.Entries...)
	sort.SliceStable(entries, func(i, j int) bool {
		leftDepth := strings.Count(entries[i].Path, "/")
		rightDepth := strings.Count(entries[j].Path, "/")
		if entries[i].Directory != entries[j].Directory {
			return entries[i].Directory
		}
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return entries[i].Path < entries[j].Path
	})
	for _, entry := range entries {
		modified := entry.Modified
		if modified.IsZero() {
			modified = time.Now()
		}
		var err error
		if entry.Directory {
			err = builder.AddDirectory(entry.Path, entry.Attributes, modified)
		} else {
			err = builder.AddData(entry.Path, entry.data, entry.Attributes, entry.Boot, modified)
		}
		if err != nil {
			return err
		}
	}
	_, err := builder.Create(path, Options{Filesystem: image.Filesystem, BootPage: image.bootPage, SectorSize: image.SectorSize, Sectors: image.Sectors})
	return err
}

// Put imports host files or directory trees into an image directory.
func (image *Image) Put(hostPaths []string, destination string) error {
	destination, err := cleanImagePath(destination)
	if err != nil {
		return err
	}
	for _, hostPath := range hostPaths {
		info, err := os.Stat(hostPath)
		if err != nil {
			return err
		}
		base := filepath.Base(hostPath)
		if info.IsDir() {
			err = filepath.WalkDir(hostPath, func(current string, item os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				relative, relErr := filepath.Rel(hostPath, current)
				if relErr != nil {
					return relErr
				}
				imagePath := joinImagePath(destination, base)
				if relative != "." {
					imagePath = joinImagePath(imagePath, filepath.ToSlash(relative))
				}
				return image.importHost(current, imagePath, item)
			})
		} else {
			err = image.importHost(hostPath, joinImagePath(destination, base), fileDirEntry{info})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type fileDirEntry struct{ os.FileInfo }

func (entry fileDirEntry) Type() os.FileMode          { return entry.Mode().Type() }
func (entry fileDirEntry) Info() (os.FileInfo, error) { return entry.FileInfo, nil }

func (image *Image) importHost(hostPath, imagePath string, item os.DirEntry) error {
	info, err := item.Info()
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links cannot be imported: %s", hostPath)
	}
	entry := ImageEntry{Path: filepath.ToSlash(imagePath), Name: filepath.Base(imagePath), Directory: info.IsDir(), Modified: info.ModTime()}
	if !entry.Directory {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", hostPath)
		}
		entry.data, err = os.ReadFile(hostPath)
		if err != nil {
			return err
		}
		entry.Size = len(entry.data)
	}
	image.replaceEntry(entry)
	return nil
}

func (image *Image) replaceEntry(entry ImageEntry) {
	for index := range image.Entries {
		if strings.EqualFold(image.Entries[index].Path, entry.Path) {
			image.Entries[index] = entry
			return
		}
	}
	image.Entries = append(image.Entries, entry)
}

// Mkdir creates an empty image directory.
func (image *Image) Mkdir(path string) error {
	clean, err := cleanImagePath(path)
	if err != nil || clean == "" {
		if err == nil {
			err = errors.New("directory path is empty")
		}
		return err
	}
	image.replaceEntry(ImageEntry{Path: clean, Name: filepath.Base(clean), Directory: true, Modified: time.Now()})
	return nil
}

// Remove deletes an entry and, for a directory, all descendants.
func (image *Image) Remove(path string) error {
	clean, err := cleanImagePath(path)
	if err != nil || clean == "" {
		return errors.New("select an ATR entry to remove")
	}
	prefix := strings.ToUpper(clean) + "/"
	output := image.Entries[:0]
	found := false
	for _, entry := range image.Entries {
		upper := strings.ToUpper(entry.Path)
		if upper == strings.ToUpper(clean) || strings.HasPrefix(upper, prefix) {
			found = true
			continue
		}
		output = append(output, entry)
	}
	if !found {
		return fmt.Errorf("ATR entry %q was not found", clean)
	}
	image.Entries = output
	return nil
}

// Rename moves or renames an entry and all directory descendants.
func (image *Image) Rename(oldPath, newPath string) error {
	oldClean, err := cleanImagePath(oldPath)
	if err != nil || oldClean == "" {
		return errors.New("select an ATR entry to rename")
	}
	newClean, err := cleanImagePath(newPath)
	if err != nil || newClean == "" {
		return errors.New("new ATR path is empty")
	}
	oldUpper, prefix := strings.ToUpper(oldClean), strings.ToUpper(oldClean)+"/"
	found := false
	for index := range image.Entries {
		upper := strings.ToUpper(image.Entries[index].Path)
		if upper == oldUpper || strings.HasPrefix(upper, prefix) {
			suffix := image.Entries[index].Path[len(oldClean):]
			image.Entries[index].Path = newClean + suffix
			image.Entries[index].Name = filepath.Base(image.Entries[index].Path)
			found = true
		}
	}
	if !found {
		return fmt.Errorf("ATR entry %q was not found", oldClean)
	}
	return nil
}

// Extract copies one image entry to an exact host path.
func (image *Image) Extract(path, output string) error {
	var selected *ImageEntry
	for index := range image.Entries {
		if strings.EqualFold(image.Entries[index].Path, path) {
			selected = &image.Entries[index]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("ATR entry %q was not found", path)
	}
	if !selected.Directory {
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return err
		}
		return os.WriteFile(output, selected.data, 0o644)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	prefix := strings.ToUpper(selected.Path) + "/"
	for _, entry := range image.Entries {
		if !strings.HasPrefix(strings.ToUpper(entry.Path), prefix) {
			continue
		}
		relative := filepath.FromSlash(entry.Path[len(selected.Path)+1:])
		target := filepath.Join(output, relative)
		if entry.Directory {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(target, entry.data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func joinImagePath(parts ...string) string {
	output := ""
	for _, part := range parts {
		part = strings.Trim(filepath.ToSlash(part), "/")
		if part == "" {
			continue
		}
		if output != "" {
			output += "/"
		}
		output += part
	}
	return output
}
