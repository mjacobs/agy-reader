package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func editionFile(name string) *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String(name),
		Package: proto.String("pkg"),
		Syntax:  proto.String("editions"),
		Edition: descriptorpb.Edition_EDITION_2024.Enum(),
	}
}

func enumWithZero(name, zero string, visibility descriptorpb.SymbolVisibility) *descriptorpb.EnumDescriptorProto {
	return &descriptorpb.EnumDescriptorProto{
		Name:       proto.String(name),
		Value:      []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String(zero), Number: proto.Int32(0)}},
		Visibility: visibility.Enum(),
	}
}

// printSet runs the command on files and returns each printed file by name.
func printSet(t *testing.T, relative bool, files ...*descriptorpb.FileDescriptorProto) map[string]string {
	t.Helper()
	data, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: files})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	set := filepath.Join(dir, "set.pb")
	if err := os.WriteFile(set, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := run(set, out, !relative); err != nil {
		t.Fatal(err)
	}
	printed := map[string]string{}
	for _, file := range files {
		text, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(file.GetName())))
		if err != nil {
			t.Fatal(err)
		}
		printed[file.GetName()] = string(text)
	}
	return printed
}

func TestPrintsLocalMessage(t *testing.T) {
	file := editionFile("pkg/schema.proto")
	file.MessageType = []*descriptorpb.DescriptorProto{
		{Name: proto.String("Item"), Visibility: descriptorpb.SymbolVisibility_VISIBILITY_LOCAL.Enum()},
	}
	want := "edition = \"2024\";\n\npackage pkg;\n\nlocal message Item {\n}\n"
	for _, relative := range []bool{false, true} {
		if got := printSet(t, relative, file)["pkg/schema.proto"]; got != want {
			t.Errorf("relative=%v: got\n%s\nwant\n%s", relative, got, want)
		}
	}
}

func TestPrintsVisibilityOnNestedAndTopLevelTypes(t *testing.T) {
	file := editionFile("pkg/schema.proto")
	file.Options = &descriptorpb.FileOptions{Features: &descriptorpb.FeatureSet{
		DefaultSymbolVisibility: descriptorpb.FeatureSet_VisibilityFeature_LOCAL_ALL.Enum(),
	}}
	file.MessageType = []*descriptorpb.DescriptorProto{
		{
			Name: proto.String("Item"),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:     proto.String("inner"),
				Number:   proto.Int32(1),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".pkg.Item.Inner"),
				JsonName: proto.String("inner"),
			}},
			NestedType: []*descriptorpb.DescriptorProto{
				{Name: proto.String("Inner"), Visibility: descriptorpb.SymbolVisibility_VISIBILITY_EXPORT.Enum()},
			},
			EnumType:   []*descriptorpb.EnumDescriptorProto{enumWithZero("Mode", "MODE_UNSPECIFIED", descriptorpb.SymbolVisibility_VISIBILITY_LOCAL)},
			Visibility: descriptorpb.SymbolVisibility_VISIBILITY_LOCAL.Enum(),
		},
		{Name: proto.String("Plain")},
	}
	file.EnumType = []*descriptorpb.EnumDescriptorProto{
		enumWithZero("Kind", "KIND_UNSPECIFIED", descriptorpb.SymbolVisibility_VISIBILITY_EXPORT),
		enumWithZero("Other", "OTHER_UNSPECIFIED", descriptorpb.SymbolVisibility_VISIBILITY_UNSET),
	}
	want := `edition = "2024";

package pkg;

option features = { default_symbol_visibility: LOCAL_ALL };

local message Item {
  .pkg.Item.Inner inner = 1;

  export message Inner {
  }

  local enum Mode {
    MODE_UNSPECIFIED = 0;
  }
}

message Plain {
}

export enum Kind {
  KIND_UNSPECIFIED = 0;
}

enum Other {
  OTHER_UNSPECIFIED = 0;
}
`
	if got := printSet(t, false, file)["pkg/schema.proto"]; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestLeavesFilesWithoutVisibilityAlone(t *testing.T) {
	file := editionFile("pkg/plain.proto")
	file.MessageType = []*descriptorpb.DescriptorProto{{Name: proto.String("Item")}}
	marks, err := markVisibility(file)
	if err != nil || marks != 0 || file.SourceCodeInfo != nil {
		t.Fatalf("marks=%d err=%v source info=%v", marks, err, file.SourceCodeInfo)
	}
	printed := []byte("message /* " + visibilityMarker + "local */ Item {\n}\n")
	if got, err := applyVisibility(printed, 0); err != nil || string(got) != string(printed) {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestRefusesWhatItCannotMark(t *testing.T) {
	file := editionFile("pkg/schema.proto")
	file.MessageType = []*descriptorpb.DescriptorProto{
		{Name: proto.String("Item"), Visibility: descriptorpb.SymbolVisibility_VISIBILITY_LOCAL.Enum()},
	}
	file.SourceCodeInfo = &descriptorpb.SourceCodeInfo{Location: []*descriptorpb.SourceCodeInfo_Location{
		{Path: []int32{4, 0, 1}, Span: []int32{1, 1, 5}, LeadingComments: proto.String(" a comment")},
	}}
	if _, err := markVisibility(file); err == nil || !strings.Contains(err.Error(), "already has a comment") {
		t.Errorf("a commented name: got %v", err)
	}
	if _, err := applyVisibility([]byte("message Item {\n}\n"), 1); err == nil || !strings.Contains(err.Error(), "0 of 1 markers") {
		t.Errorf("a missing marker: got %v", err)
	}
}
