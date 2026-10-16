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
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
		handle, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		if err := printer.PrintProtoFile(files[name], handle); err != nil {
			handle.Close()
			return fmt.Errorf("print %s: %w", name, err)
		}
		if err := handle.Close(); err != nil {
			return err
		}
	}
	fmt.Println("Printed", len(set.File), "proto files")
	return nil
}
