package appicon

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
)

// ICNS wraps PNG images in the macOS icon container, including Retina sizes.
func ICNS() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("icns\x00\x00\x00\x00")
	for i, size := range []int{128, 256, 512, 1024} {
		png, err := PNG(size)
		if err != nil {
			return nil, err
		}
		b.WriteString([]string{"ic07", "ic08", "ic09", "ic10"}[i])
		binary.Write(&b, binary.BigEndian, uint32(len(png)+8))
		b.Write(png)
	}
	data := b.Bytes()
	binary.BigEndian.PutUint32(data[4:], uint32(len(data)))
	return data, nil
}

// WindowsResource makes an amd64 COFF object that Go's linker embeds as .rsrc.
// The two resource types are RT_ICON (3) and RT_GROUP_ICON (14), each with ID 1.
func WindowsResource() ([]byte, error) {
	png, err := PNG(256)
	if err != nil {
		return nil, err
	}
	groupOffset := (160 + len(png) + 3) &^ 3
	data := make([]byte, groupOffset+20)
	put := func(offset int, value uint32) { binary.LittleEndian.PutUint32(data[offset:], value) }
	// Resource directory: type -> ID -> neutral language -> data entry.
	for _, offset := range []int{0, 32, 56, 80, 104} {
		binary.LittleEndian.PutUint16(data[offset+14:], 1)
	}
	binary.LittleEndian.PutUint16(data[14:], 2)
	for _, entry := range [][3]uint32{
		{16, 3, 0x80000020}, {24, 14, 0x80000038},
		{48, 1, 0x80000050}, {72, 1, 0x80000068},
		{96, 0, 128}, {120, 0, 144},
	} {
		put(int(entry[0]), entry[1])
		put(int(entry[0])+4, entry[2])
	}
	put(128, 160)
	put(132, uint32(len(png)))
	put(144, uint32(groupOffset))
	put(148, 20)
	copy(data[160:], png)
	var group bytes.Buffer
	// GRPICONDIR and its entry: 0 means 256 pixels; the final word is the RT_ICON ID.
	binary.Write(&group, binary.LittleEndian, []uint16{0, 1, 1, 0, 0, 1, 32})
	binary.Write(&group, binary.LittleEndian, uint32(len(png)))
	binary.Write(&group, binary.LittleEndian, uint16(1))
	copy(data[groupOffset:], group.Bytes())

	var b bytes.Buffer
	const dataOffset = 20 + 40 // COFF header and one section header
	relocOffset := uint32(dataOffset + len(data))
	binary.Write(&b, binary.LittleEndian, pe.FileHeader{
		Machine: pe.IMAGE_FILE_MACHINE_AMD64, NumberOfSections: 1,
		PointerToSymbolTable: relocOffset + 20, NumberOfSymbols: 1,
	})
	binary.Write(&b, binary.LittleEndian, pe.SectionHeader32{
		Name: [8]uint8{'.', 'r', 's', 'r', 'c'}, SizeOfRawData: uint32(len(data)),
		PointerToRawData: dataOffset, PointerToRelocations: relocOffset, NumberOfRelocations: 2,
		Characteristics: pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ,
	})
	b.Write(data)
	for _, offset := range []uint32{128, 144} {
		binary.Write(&b, binary.LittleEndian, pe.Reloc{VirtualAddress: offset, Type: 3}) // IMAGE_REL_AMD64_ADDR32NB
	}
	binary.Write(&b, binary.LittleEndian, pe.COFFSymbol{
		Name: [8]uint8{'.', 'r', 's', 'r', 'c'}, SectionNumber: 1, StorageClass: 3,
	})
	binary.Write(&b, binary.LittleEndian, uint32(4)) // empty COFF string table
	return b.Bytes(), nil
}
