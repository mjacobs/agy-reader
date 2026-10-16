package main
import("os";"path/filepath";"fmt";"github.com/jhump/protoreflect/desc";"github.com/jhump/protoreflect/desc/protoprint";"google.golang.org/protobuf/proto";"google.golang.org/protobuf/types/descriptorpb")
func main(){ b,e:=os.ReadFile(os.Args[1]);must(e); var s descriptorpb.FileDescriptorSet;must(proto.Unmarshal(b,&s)); fs,e:=desc.CreateFileDescriptorsFromSet(&s);must(e);p:=protoprint.Printer{};for _,f:=range s.File{fd:=fs[f.GetName()];out:=filepath.Join(os.Args[2],f.GetName());must(os.MkdirAll(filepath.Dir(out),0755));h,e:=os.Create(out);must(e);must(p.PrintProtoFile(fd,h));must(h.Close())};fmt.Println("Printed",len(s.File),"proto files")}
func must(e error){if e!=nil{panic(e)}}
