// Command proto-printer writes one readable .proto file for each file in a
// binary FileDescriptorSet. It is a separate module so that the reader itself
// does not depend on protoreflect.
//
// Usage: proto-printer [-relative] DESCRIPTOR_SET OUT_DIR
//
// OUT_DIR must not exist. Output paths come from the descriptor file names;
// a name that is absolute or climbs out of OUT_DIR is refused.
//
// By default every type reference is fully qualified (.pkg.Message), which is
// never ambiguous. -relative makes references as short as protoprint can,
// which reads better but can change meaning: a nested message that shares a
// name with an outer message, or with the first part of an outer name,
// captures a short reference meant for the outer one. Compile -relative
// output back and compare it, as scripts/agy_proto_snapshot.py does.
//
// protoprint does not write Edition 2024 visibility (export or local on a
// message or enum), so this command adds the keyword itself; see markVisibility.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoprint"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func main() {
	relative := flag.Bool("relative", false, "shorten type references; check the result by compiling it back")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: proto-printer [-relative] DESCRIPTOR_SET OUT_DIR")
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), flag.Arg(1), !*relative); err != nil {
		fmt.Fprintln(os.Stderr, "proto-printer:", err)
		os.Exit(1)
	}
}

func run(setPath, outDir string, qualified bool) error {
	data, err := os.ReadFile(setPath)
	if err != nil {
		return err
	}
	var set descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &set); err != nil {
		return fmt.Errorf("parse %s: %w", setPath, err)
	}
	marks := make(map[string]int, len(set.File))
	for _, file := range set.File {
		if marks[file.GetName()], err = markVisibility(file); err != nil {
			return fmt.Errorf("print %s: %w", file.GetName(), err)
		}
	}
	files, err := desc.CreateFileDescriptorsFromSet(&set)
	if err != nil {
		return err
	}
	if err := os.Mkdir(outDir, 0o755); err != nil {
		return err
	}
	printer := protoprint.Printer{ForceFullyQualifiedNames: qualified}
	for _, file := range set.File {
		name := file.GetName()
		clean := filepath.Clean(filepath.FromSlash(name))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("refusing descriptor name outside the output directory: %q", name)
		}
		out := filepath.Join(outDir, clean)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		var text bytes.Buffer
		if err := printer.PrintProtoFile(files[name], &text); err != nil {
			return fmt.Errorf("print %s: %w", name, err)
		}
		printed, err := applyVisibility(text.Bytes(), marks[name])
		if err != nil {
			return fmt.Errorf("print %s: %w", name, err)
		}
		handle, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		if _, err := handle.Write(printed); err != nil {
			handle.Close()
			return err
		}
		if err := handle.Close(); err != nil {
			return err
		}
	}
	fmt.Println("Printed", len(set.File), "proto files")
	return nil
}

// protoprint (jhump/protoreflect v1.19.0, and v2.0.0) ignores the visibility
// field of DescriptorProto and EnumDescriptorProto, so `local message M {}`
// would print as `message M {}` and compile back to a different descriptor.
//
// protoprint prints a type's name through the source location of the name
// itself (the type's path plus its name field), and prints a leading comment
// there inline: `message /* ... */ M {`. protoc never attaches comments to a
// name, and element order depends only on the locations of whole elements, so
// a marker comment there changes nothing else. markVisibility adds those
// markers; applyVisibility turns each into the keyword before the declaration.
// No other byte of the output changes.

const (
	descriptorMessageTag = 4 // FileDescriptorProto.message_type
	descriptorEnumTag    = 5 // FileDescriptorProto.enum_type
	nestedMessageTag     = 3 // DescriptorProto.nested_type
	nestedEnumTag        = 4 // DescriptorProto.enum_type
	nameTag              = 1 // DescriptorProto.name and EnumDescriptorProto.name

	visibilityMarker = "agy-proto-printer visibility="
)

var markedDeclaration = regexp.MustCompile(`(message|enum) /\* ` + visibilityMarker + `(export|local) \*/ `)

// markVisibility adds a marker comment for each message and enum in file that
// sets a visibility, and returns how many it added.
func markVisibility(file *descriptorpb.FileDescriptorProto) (int, error) {
	marks := 0
	var mark func(path []int32, visibility descriptorpb.SymbolVisibility) error
	mark = func(path []int32, visibility descriptorpb.SymbolVisibility) error {
		var keyword string
		switch visibility {
		case descriptorpb.SymbolVisibility_VISIBILITY_UNSET:
			return nil
		case descriptorpb.SymbolVisibility_VISIBILITY_EXPORT:
			keyword = "export"
		case descriptorpb.SymbolVisibility_VISIBILITY_LOCAL:
			keyword = "local"
		default:
			return fmt.Errorf("unknown symbol visibility %d at %v", visibility, path)
		}
		namePath := append(slices.Clone(path), nameTag)
		comment := " " + visibilityMarker + keyword
		if file.SourceCodeInfo == nil {
			file.SourceCodeInfo = &descriptorpb.SourceCodeInfo{}
		}
		// protoprint uses the first location for a path; reuse it if present.
		for _, loc := range file.SourceCodeInfo.Location {
			if slices.Equal(loc.Path, namePath) {
				if loc.GetLeadingComments() != "" {
					return fmt.Errorf("cannot mark visibility: the name at %v already has a comment", path)
				}
				loc.LeadingComments = &comment
				marks++
				return nil
			}
		}
		file.SourceCodeInfo.Location = append(file.SourceCodeInfo.Location, &descriptorpb.SourceCodeInfo_Location{
			Path:            namePath,
			Span:            []int32{0, 0, 0},
			LeadingComments: &comment,
		})
		marks++
		return nil
	}
	var walk func(path []int32, message *descriptorpb.DescriptorProto) error
	walk = func(path []int32, message *descriptorpb.DescriptorProto) error {
		if err := mark(path, message.GetVisibility()); err != nil {
			return err
		}
		for i, nested := range message.NestedType {
			if err := walk(append(slices.Clone(path), nestedMessageTag, int32(i)), nested); err != nil {
				return err
			}
		}
		for i, enum := range message.EnumType {
			if err := mark(append(slices.Clone(path), nestedEnumTag, int32(i)), enum.GetVisibility()); err != nil {
				return err
			}
		}
		return nil
	}
	for i, message := range file.MessageType {
		if err := walk([]int32{descriptorMessageTag, int32(i)}, message); err != nil {
			return 0, err
		}
	}
	for i, enum := range file.EnumType {
		if err := mark([]int32{descriptorEnumTag, int32(i)}, enum.GetVisibility()); err != nil {
			return 0, err
		}
	}
	return marks, nil
}

// applyVisibility replaces each marker in printed with the keyword, placed
// before `message` or `enum`. It refuses unless it finds exactly marks markers,
// so a type protoprint did not print as a plain declaration is never lost.
func applyVisibility(printed []byte, marks int) ([]byte, error) {
	if marks == 0 {
		return printed, nil
	}
	found := len(markedDeclaration.FindAllIndex(printed, -1))
	if found != marks {
		return nil, fmt.Errorf("cannot write symbol visibility: %d of %d markers printed as declarations", found, marks)
	}
	return markedDeclaration.ReplaceAll(printed, []byte("$2 $1 ")), nil
}
