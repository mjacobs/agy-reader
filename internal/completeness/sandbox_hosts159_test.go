package completeness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/testfixture"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const sandboxHosts159Canonical = `{"cascadeId":"sidecar-allowlist","metadata":{"sourceMetadata":{"sidecar":{"sidecarId":"fixture","config":{"sandbox":{"allowedHttpHosts":["example.com"]}}}}},"steps":[]}`

func sandboxSnapshot159(t *testing.T, raw string) *Snapshot {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.db")
	testfixture.Source(t, path, []byte(raw))
	s, err := ReadSnapshot(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSandboxHosts159AllowlistVerification(t *testing.T) {
	s := sandboxSnapshot159(t, sandboxHosts159Canonical)

	t.Run("issue scenario", func(t *testing.T) {
		rpc := strings.Replace(sandboxHosts159Canonical, "allowedHttpHosts", "allowedHosts", 1)
		if r := s.CompareFullRaw([]byte(rpc)); r != (Result{State: Verified}) {
			t.Fatalf("CompareFullRaw = %+v, want Verified", r)
		}
		if r := s.ComparePresentation([]byte(rpc)); r != (Result{State: Verified}) {
			t.Fatalf("ComparePresentation = %+v, want Verified", r)
		}
		if r := s.CompareResponses([]byte(rpc), []byte(rpc)); r != (Result{State: Verified}) {
			t.Fatalf("CompareResponses = %+v, want Verified", r)
		}
		a, r := s.VerifyResponses([]byte(rpc), []byte(rpc))
		if r != (Result{State: Verified}) || a.Reason != "" || len(a.SourceOnly) != 0 {
			t.Fatalf("VerifyResponses = %+v, %+v; want plain Verified", a, r)
		}
		if r := s.CheckAttestation([]byte(rpc), a); r != (Result{State: Verified}) {
			t.Fatalf("CheckAttestation = %+v, want Verified", r)
		}
		// Sidecars are re-indented and carry agyReader.completeness on disk.
		var sidecar map[string]any
		if err := json.Unmarshal([]byte(rpc), &sidecar); err != nil {
			t.Fatal(err)
		}
		sidecar["agyReader"] = map[string]any{"completeness": a}
		indented, err := json.MarshalIndent(sidecar, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		saved := ReadAttestation(indented)
		if r := s.CheckAttestation(indented, saved); r != (Result{State: Verified}) {
			t.Fatalf("CheckAttestation(indented) = %+v, want Verified", r)
		}
		offline, err := s.OfflineJSON()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(offline, []byte(`"allowedHttpHosts"`)) || bytes.Contains(offline, []byte(`"allowedHosts"`)) {
			t.Fatalf("OfflineJSON did not preserve canonical allowedHttpHosts:\n%s", offline)
		}
		if r := s.ComparePresentation(offline); r != (Result{State: Verified}) {
			t.Fatalf("ComparePresentation(OfflineJSON) = %+v, want Verified", r)
		}
	})

	for _, tc := range []struct {
		name string
		key  string
	}{
		{"camelCase legacy", `"allowedHosts"`},
		{"snake_case legacy", `"allowed_hosts"`},
		{"unicode escaped legacy", `"allowed\u0048osts"`},
		{"unicode escaped snake legacy", `"allowed\u005fhosts"`},
		{"camelCase canonical", `"allowedHttpHosts"`},
		{"snake_case canonical", `"allowed_http_hosts"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			persisted := `{"cascadeId":"sidecar-allowlist","metadata":{"sourceMetadata":{"sidecar":{"sidecarId":"fixture","config":{"sandbox":{"disableSandbox":false,"allowFullNetwork":true,"readOnlyPaths":["/ro"],"readWritePaths":["/rw"],"allowedHttpHosts":["example.com","api.example.org"]}}}}},"steps":[]}`
			snap := sandboxSnapshot159(t, persisted)
			rpc := strings.Replace(persisted, `"allowedHttpHosts"`, tc.key, 1)
			a, r := snap.VerifyResponses([]byte(rpc), []byte(rpc))
			if r != (Result{State: Verified}) || a.Reason != "" || len(a.SourceOnly) != 0 {
				t.Fatalf("VerifyResponses(%s) = %+v, %+v; want Verified", tc.name, a, r)
			}
			if r := snap.CheckAttestation([]byte(rpc), a); r != (Result{State: Verified}) {
				t.Fatalf("CheckAttestation(%s) = %+v, want Verified", tc.name, r)
			}
		})
	}
}

func TestSandboxHosts159RejectsMismatches(t *testing.T) {
	s := sandboxSnapshot159(t, sandboxHosts159Canonical)
	legacyRPC := strings.Replace(sandboxHosts159Canonical, "allowedHttpHosts", "allowedHosts", 1)

	for _, tc := range []struct {
		name string
		rpc  string
	}{
		{"altered host", strings.Replace(legacyRPC, `"example.com"`, `"other.example"`, 1)},
		{"extra host", strings.Replace(legacyRPC, `["example.com"]`, `["example.com","other.example"]`, 1)},
		{"empty list", strings.Replace(legacyRPC, `["example.com"]`, `[]`, 1)},
		{"null list", strings.Replace(legacyRPC, `["example.com"]`, `null`, 1)},
		{"omitted list", strings.Replace(legacyRPC, `"allowedHosts":["example.com"]`, ``, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if r := s.CompareFullRaw([]byte(tc.rpc)); r != (Result{State: Incomplete, Reason: PersistedRPCMismatch}) {
				t.Fatalf("CompareFullRaw = %+v, want Incomplete/%q", r, PersistedRPCMismatch)
			}
			if a, r := s.VerifyResponses([]byte(tc.rpc), []byte(legacyRPC)); r.State != Incomplete || a != nil {
				t.Fatalf("VerifyResponses(full mismatch) = %+v, %+v; want Incomplete", a, r)
			}
			if a, r := s.VerifyResponses([]byte(legacyRPC), []byte(tc.rpc)); r.State != Incomplete || a != nil {
				t.Fatalf("VerifyResponses(pres mismatch) = %+v, %+v; want Incomplete", a, r)
			}
		})
	}

	multiPersisted := `{"cascadeId":"sidecar-allowlist","metadata":{"sourceMetadata":{"sidecar":{"sidecarId":"fixture","config":{"sandbox":{"allowedHttpHosts":["a.example","b.example"]}}}}},"steps":[]}`
	multiSnap := sandboxSnapshot159(t, multiPersisted)
	for _, tc := range []struct {
		name string
		rpc  string
	}{
		{"missing host", strings.Replace(multiPersisted, `"allowedHttpHosts":["a.example","b.example"]`, `"allowedHosts":["a.example"]`, 1)},
		{"reordered hosts", strings.Replace(multiPersisted, `"allowedHttpHosts":["a.example","b.example"]`, `"allowedHosts":["b.example","a.example"]`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if r := multiSnap.CompareFullRaw([]byte(tc.rpc)); r != (Result{State: Incomplete, Reason: PersistedRPCMismatch}) {
				t.Fatalf("CompareFullRaw = %+v, want Incomplete", r)
			}
		})
	}

	emptyPersisted := `{"cascadeId":"sidecar-allowlist","metadata":{"sourceMetadata":{"sidecar":{"sidecarId":"fixture","config":{"sandbox":{}}}}},"steps":[]}`
	emptySnap := sandboxSnapshot159(t, emptyPersisted)
	nonEmptyLegacy := strings.Replace(emptyPersisted, `"sandbox":{}`, `"sandbox":{"allowedHosts":["example.com"]}`, 1)
	if r := emptySnap.CompareFullRaw([]byte(nonEmptyLegacy)); r != (Result{State: Incomplete, Reason: PersistedRPCMismatch}) {
		t.Fatalf("empty persisted vs nonempty allowedHosts = %+v, want Incomplete", r)
	}
	for _, emptyVal := range []string{`[]`, `null`} {
		emptyLegacy := strings.Replace(emptyPersisted, `"sandbox":{}`, `"sandbox":{"allowedHosts":`+emptyVal+`}`, 1)
		if _, r := emptySnap.VerifyResponses([]byte(emptyLegacy), []byte(emptyLegacy)); r != (Result{State: Verified}) {
			t.Fatalf("empty persisted vs %s = %+v, want Verified", emptyVal, r)
		}
	}

	// Tampered sidecar and forged attestation payload digest checks.
	a, r := s.VerifyResponses([]byte(legacyRPC), []byte(legacyRPC))
	if r != (Result{State: Verified}) {
		t.Fatal(r)
	}
	tampered := strings.Replace(legacyRPC, `"example.com"`, `"evil.example"`, 1)
	if r := s.CheckAttestation([]byte(tampered), a); r.State != Incomplete {
		t.Fatalf("tampered sidecar CheckAttestation = %+v, want Incomplete", r)
	}
	forgedDigest, err := PayloadDigest([]byte(tampered))
	if err != nil {
		t.Fatal(err)
	}
	forged := *a
	forged.PayloadDigest = forgedDigest
	if r := s.CheckAttestation([]byte(tampered), &forged); r.State != Incomplete {
		t.Fatalf("forged digest CheckAttestation = %+v, want Incomplete", r)
	}
}

func TestSandboxHosts159RejectsConflictsAndMalformed(t *testing.T) {
	s := sandboxSnapshot159(t, sandboxHosts159Canonical)

	for _, tc := range []struct {
		name    string
		sandbox string
	}{
		{"allowedHosts and allowedHttpHosts same", `{"allowedHosts":["example.com"],"allowedHttpHosts":["example.com"]}`},
		{"allowedHosts and allowedHttpHosts different", `{"allowedHosts":["example.com"],"allowedHttpHosts":["other.example"]}`},
		{"allowedHosts and null allowedHttpHosts", `{"allowedHosts":["example.com"],"allowedHttpHosts":null}`},
		{"null allowedHosts and allowedHttpHosts", `{"allowedHosts":null,"allowedHttpHosts":["example.com"]}`},
		{"allowedHosts and allowed_http_hosts", `{"allowedHosts":["example.com"],"allowed_http_hosts":["example.com"]}`},
		{"allowed_hosts and allowedHttpHosts", `{"allowed_hosts":["example.com"],"allowedHttpHosts":["example.com"]}`},
		{"allowedHosts and allowed_hosts", `{"allowedHosts":["example.com"],"allowed_hosts":["example.com"]}`},
		{"duplicate allowedHosts", `{"allowedHosts":["example.com"],"allowedHosts":["example.com"]}`},
		{"escaped and unescaped allowedHosts", `{"allowed\u0048osts":["example.com"],"allowedHosts":["example.com"]}`},
		{"allowedHosts and allowedRpcs", `{"allowedHosts":["example.com"],"allowedRpcs":["example.Read"]}`},
		{"allowedHosts and empty allowedRpcs", `{"allowedHosts":["example.com"],"allowedRpcs":[]}`},
		{"allowedHosts and null allowedRpcs", `{"allowedHosts":["example.com"],"allowedRpcs":null}`},
		{"allowedHosts and allowed_rpcs", `{"allowedHosts":["example.com"],"allowed_rpcs":["example.Read"]}`},
		{"allowed_hosts and allowedRpcs", `{"allowed_hosts":["example.com"],"allowedRpcs":["example.Read"]}`},
		{"wrong type string", `{"allowedHosts":"example.com"}`},
		{"wrong type number elem", `{"allowedHosts":[123]}`},
		{"malformed surrogate in allowedHosts", `{"allowedHosts":["\ud800"]}`},
		{"duplicate sibling field", `{"disableSandbox":false,"disableSandbox":false,"allowedHosts":["example.com"]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rpc := strings.Replace(sandboxHosts159Canonical, `{"allowedHttpHosts":["example.com"]}`, tc.sandbox, 1)
			if r := s.CompareFullRaw([]byte(rpc)); r.State != Unknown {
				t.Fatalf("CompareFullRaw(%s) = %+v, want Unknown", tc.name, r)
			}
			if _, r := s.VerifyResponses([]byte(rpc), []byte(rpc)); r.State != Unknown {
				t.Fatalf("VerifyResponses(%s) = %+v, want Unknown", tc.name, r)
			}
		})
	}

	t.Run("duplicate sandbox object", func(t *testing.T) {
		rpc := strings.Replace(
			sandboxHosts159Canonical,
			`"sandbox":{"allowedHttpHosts":["example.com"]}`,
			`"sandbox":{"allowedHosts":["example.com"]},"sandbox":{"allowedHttpHosts":["example.com"]}`,
			1,
		)
		if r := s.CompareFullRaw([]byte(rpc)); r.State != Unknown {
			t.Fatalf("CompareFullRaw(duplicate sandbox) = %+v, want Unknown", r)
		}
	})
}

func TestSandboxHosts159RejectsDuplicateKeysInSidecarAndTopLevel(t *testing.T) {
	s := sandboxSnapshot159(t, sandboxHosts159Canonical)
	legacyRPC := strings.Replace(sandboxHosts159Canonical, "allowedHttpHosts", "allowedHosts", 1)
	a, r := s.VerifyResponses([]byte(legacyRPC), []byte(legacyRPC))
	if r != (Result{State: Verified}) || a == nil {
		t.Fatalf("VerifyResponses = %+v, %+v", a, r)
	}

	for _, tc := range []struct {
		name    string
		sandbox string
	}{
		{"duplicate allowedHosts", `{"allowedHosts":["other.example"],"allowedHosts":["example.com"]}`},
		{"escaped duplicate allowedHosts", `{"allowed\u0048osts":["other.example"],"allowedHosts":["example.com"]}`},
	} {
		t.Run("saved "+tc.name, func(t *testing.T) {
			edited := strings.Replace(legacyRPC, `{"allowedHosts":["example.com"]}`, tc.sandbox, 1)
			if _, err := PayloadDigest([]byte(edited)); err == nil {
				t.Fatalf("PayloadDigest(%s) succeeded, want error", tc.name)
			}
			if r := s.CheckAttestation([]byte(edited), a); r.State != Unknown {
				t.Fatalf("CheckAttestation(%s) = %+v, want Unknown", tc.name, r)
			}
			if r := s.ComparePresentation([]byte(edited)); r.State != Unknown {
				t.Fatalf("ComparePresentation(%s) = %+v, want Unknown", tc.name, r)
			}
		})
	}

	t.Run("saved conflicting distinct keys with forged digest", func(t *testing.T) {
		edited := strings.Replace(
			legacyRPC,
			`{"allowedHosts":["example.com"]}`,
			`{"allowedHosts":["other.example"],"allowedHttpHosts":["example.com"]}`,
			1,
		)
		forgedDigest, err := PayloadDigest([]byte(edited))
		if err != nil {
			t.Fatal(err)
		}
		forged := *a
		forged.PayloadDigest = forgedDigest
		if r := s.CheckAttestation([]byte(edited), &forged); r.State != Unknown {
			t.Fatalf("CheckAttestation(forged conflict) = %+v, want Unknown", r)
		}
	})

	const matchingMeta = `"metadata":{"sourceMetadata":{"sidecar":{"sidecarId":"fixture","config":{"sandbox":{"allowedHosts":["example.com"]}}}}}`
	const conflictingMeta = `"metadata":{"sourceMetadata":{"sidecar":{"sidecarId":"fixture","config":{"sandbox":{"allowedHosts":["other.example"],"allowedHttpHosts":["example.com"]}}}}}`
	for _, tc := range []struct {
		name string
		rpc  string
	}{
		{
			"duplicated metadata",
			strings.Replace(legacyRPC, matchingMeta, conflictingMeta+","+matchingMeta, 1),
		},
		{
			"unicode-escaped duplicated metadata",
			strings.Replace(legacyRPC, matchingMeta, strings.Replace(conflictingMeta, `"metadata"`, `"metadat\u0061"`, 1)+","+matchingMeta, 1),
		},
		{
			"duplicated agyReader",
			strings.Replace(legacyRPC, `"steps":[]`, `"agyReader":{},"agyReader":{},"steps":[]`, 1),
		},
	} {
		t.Run("live "+tc.name, func(t *testing.T) {
			if r := s.CompareFullRaw([]byte(tc.rpc)); r.State != Unknown {
				t.Fatalf("CompareFullRaw(%s) = %+v, want Unknown", tc.name, r)
			}
			if got, r := s.VerifyResponses([]byte(tc.rpc), []byte(legacyRPC)); r.State != Unknown || got != nil {
				t.Fatalf("VerifyResponses(%s) = %+v, %+v; want Unknown/nil", tc.name, got, r)
			}
		})
	}
}

func TestSandboxHosts159IgnoresUnrelatedFields(t *testing.T) {
	raw := `{"cascadeId":"sidecar-allowlist","metadata":{"sourceMetadata":{"sidecar":{"sidecarId":"fixture","config":{"sandbox":{"allowedHttpHosts":["example.com"]}}}}},"steps":[{"type":"CORTEX_STEP_TYPE_USER_INPUT","userInput":{"items":[{"text":"hi"}]}},{"type":"CORTEX_STEP_TYPE_RPC_ACTION","rpcAction":{"arguments":{"allowedHosts":["a"],"allowedHttpHosts":["b"],"allowedRpcs":["c"]}}}]}`
	s := sandboxSnapshot159(t, raw)

	rpc := strings.Replace(raw, `"config":{"sandbox":{"allowedHttpHosts":["example.com"]}}`, `"config":{"sandbox":{"allowedHosts":["example.com"]}}`, 1)
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rpc), &obj); err != nil {
		t.Fatal(err)
	}
	obj["future"] = json.RawMessage(`{"allowedHosts":["x"],"allowedHttpHosts":["y"],"allowedRpcs":["z"]}`)
	withFuture, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	a, r := s.VerifyResponses(withFuture, withFuture)
	if r != (Result{State: Verified}) {
		t.Fatalf("VerifyResponses = %+v, want Verified", r)
	}
	if r := s.CheckAttestation(withFuture, a); r != (Result{State: Verified}) {
		t.Fatalf("CheckAttestation = %+v, want Verified", r)
	}
	decoded, err := rpcMessage(withFuture)
	if err != nil {
		t.Fatal(err)
	}
	steps := decoded.ProtoReflect().Get(decoded.ProtoReflect().Descriptor().Fields().ByName("steps")).List()
	rpcAction := steps.Get(1).Message().Get(steps.Get(1).Message().Descriptor().Fields().ByName("rpc_action")).Message()
	argsBytes, err := protojson.MarshalOptions{Resolver: schemaTypes}.Marshal(rpcAction.Get(rpcAction.Descriptor().Fields().ByName("arguments")).Message().Interface())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(argsBytes, []byte(`"allowedHosts"`)) || !bytes.Contains(argsBytes, []byte(`"allowedHttpHosts"`)) {
		t.Fatalf("Struct rpcAction.arguments was modified: %s", argsBytes)
	}

	// google.protobuf.Any wrapping SidecarSandboxConfig directly and via nested Any.
	directAny := `{"cascadeId":"any-sandbox","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","generic":{"result":{"payload":{"@type":"type.googleapis.com/exa.cortex_pb.SidecarSandboxConfig","allowedHosts":["example.com"]}}}}]}`
	msg, err := rpcMessage([]byte(directAny))
	if err != nil {
		t.Fatalf("direct Any rpcMessage failed: %v", err)
	}
	anyStep := msg.ProtoReflect().Get(msg.ProtoReflect().Descriptor().Fields().ByName("steps")).List().Get(0).Message()
	anyPayload := anyStep.Get(anyStep.Descriptor().Fields().ByName("generic")).Message()
	anyRes := anyPayload.Get(anyPayload.Descriptor().Fields().ByName("result")).Message()
	anyMsg := anyRes.Get(anyRes.Descriptor().Fields().ByName("payload")).Message()
	unpacked, err := decode("exa.cortex_pb.SidecarSandboxConfig", anyMsg.Get(anyMsg.Descriptor().Fields().ByName("value")).Bytes())
	if err != nil {
		t.Fatal(err)
	}
	hosts := unpacked.ProtoReflect().Get(unpacked.ProtoReflect().Descriptor().Fields().ByName("allowed_http_hosts")).List()
	if hosts.Len() != 1 || hosts.Get(0).String() != "example.com" {
		t.Fatalf("direct Any allowed_http_hosts len=%d, want [example.com]", hosts.Len())
	}

	nestedAny := `{"cascadeId":"nested-any","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","generic":{"result":{"payload":{"@type":"type.googleapis.com/google.protobuf.Any","value":{"@type":"type.googleapis.com/exa.cortex_pb.SidecarSandboxConfig","allowedHosts":["nested.example"]}}}}}]}`
	nestedMsg, err := rpcMessage([]byte(nestedAny))
	if err != nil {
		t.Fatalf("nested Any rpcMessage failed: %v", err)
	}
	nestedStep := nestedMsg.ProtoReflect().Get(nestedMsg.ProtoReflect().Descriptor().Fields().ByName("steps")).List().Get(0).Message()
	outerAny := nestedStep.Get(nestedStep.Descriptor().Fields().ByName("generic")).Message()
	outerAnyRes := outerAny.Get(outerAny.Descriptor().Fields().ByName("result")).Message()
	outerAnyMsg := outerAnyRes.Get(outerAnyRes.Descriptor().Fields().ByName("payload")).Message()
	innerAny, err := decode("google.protobuf.Any", outerAnyMsg.Get(outerAnyMsg.Descriptor().Fields().ByName("value")).Bytes())
	if err != nil {
		t.Fatal(err)
	}
	innerUnpacked, err := decode("exa.cortex_pb.SidecarSandboxConfig", innerAny.ProtoReflect().Get(innerAny.ProtoReflect().Descriptor().Fields().ByName("value")).Bytes())
	if err != nil {
		t.Fatal(err)
	}
	nestedHosts := innerUnpacked.ProtoReflect().Get(innerUnpacked.ProtoReflect().Descriptor().Fields().ByName("allowed_http_hosts")).List()
	if nestedHosts.Len() != 1 || nestedHosts.Get(0).String() != "nested.example" {
		t.Fatalf("nested Any allowed_http_hosts len=%d, want [nested.example]", nestedHosts.Len())
	}

	conflictingAny := strings.Replace(directAny, `"allowedHosts":["example.com"]`, `"allowedHosts":["example.com"],"allowedHttpHosts":["example.com"]`, 1)
	if _, err := rpcMessage([]byte(conflictingAny)); err == nil {
		t.Fatal("conflicting Any allowedHosts + allowedHttpHosts accepted")
	}
}

func TestSandboxHosts159DescriptorProvenance(t *testing.T) {
	loadSandbox := func(dir, digest string) *descriptorpb.DescriptorProto {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "artifacts", "proto-snapshots", dir, "closure.protoset"))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != digest {
			t.Fatalf("%s closure.protoset hash = %s, want %s", dir, hex.EncodeToString(sum[:]), digest)
		}
		var set descriptorpb.FileDescriptorSet
		if err := proto.Unmarshal(data, &set); err != nil {
			t.Fatal(err)
		}
		for _, f := range set.File {
			if f.GetPackage() != "exa.cortex_pb" {
				continue
			}
			for _, m := range f.MessageType {
				if m.GetName() == "SidecarSandboxConfig" {
					return m
				}
			}
		}
		t.Fatalf("%s missing SidecarSandboxConfig", dir)
		return nil
	}
	fieldByNumber := func(d *descriptorpb.DescriptorProto, num int32) *descriptorpb.FieldDescriptorProto {
		for _, f := range d.Field {
			if f.GetNumber() == num {
				return f
			}
		}
		return nil
	}

	d159 := loadSandbox("agy-1.5.9-39816d49d5ab", "2134e03f83ba5d97a3fd8b5fa7574154be0cb71294e373a90a5cad3fd7e481d2")
	f5_159 := fieldByNumber(d159, 5)
	if f5_159 == nil || f5_159.GetName() != "allowed_hosts" || f5_159.GetJsonName() != "allowedHosts" ||
		f5_159.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED || f5_159.GetType() != descriptorpb.FieldDescriptorProto_TYPE_STRING {
		t.Fatalf("1.5.9 field 5 = %+v", f5_159)
	}
	if fieldByNumber(d159, 6) != nil {
		t.Fatal("1.5.9 unexpectedly defines field 6")
	}

	d1510 := loadSandbox("agy-1.5.10-20d6f74672ce", "5e2adf25951e87fe151b3fd097ab2c772990ecfb167f3b2025cab40119aae1b1")
	f5_1510 := fieldByNumber(d1510, 5)
	if f5_1510 == nil || f5_1510.GetName() != "allowed_http_hosts" || f5_1510.GetJsonName() != "allowedHttpHosts" ||
		f5_1510.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED || f5_1510.GetType() != descriptorpb.FieldDescriptorProto_TYPE_STRING {
		t.Fatalf("1.5.10 field 5 = %+v", f5_1510)
	}
	if fieldByNumber(d1510, 6) != nil {
		t.Fatal("1.5.10 unexpectedly defines field 6")
	}

	d1514 := loadSandbox("agy-1.5.14-37873b806239", "a48906e588f470c2d37db003a5ed41a7a9989bfa7bdfb562d4b698f60af038dd")
	f5_1514 := fieldByNumber(d1514, 5)
	f6_1514 := fieldByNumber(d1514, 6)
	if f5_1514 == nil || f5_1514.GetName() != "allowed_http_hosts" || f5_1514.GetJsonName() != "allowedHttpHosts" ||
		f5_1514.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED || f5_1514.GetType() != descriptorpb.FieldDescriptorProto_TYPE_STRING {
		t.Fatalf("1.5.14 field 5 = %+v", f5_1514)
	}
	if f6_1514 == nil || f6_1514.GetName() != "allowed_rpcs" || f6_1514.GetJsonName() != "allowedRpcs" {
		t.Fatalf("1.5.14 field 6 = %+v", f6_1514)
	}

	// Ensure embedded schema matches field 5 as canonical allowed_http_hosts.
	if err := loadSchema(); err != nil {
		t.Fatal(err)
	}
	msg, err := newMessage("exa.cortex_pb.SidecarSandboxConfig")
	if err != nil {
		t.Fatal(err)
	}
	f5 := msg.ProtoReflect().Descriptor().Fields().ByNumber(5)
	if f5 == nil || f5.Name() != "allowed_http_hosts" || f5.JSONName() != "allowedHttpHosts" ||
		f5.Cardinality() != protoreflect.Repeated || f5.Kind() != protoreflect.StringKind {
		t.Fatalf("embedded SidecarSandboxConfig field 5 = %+v", f5)
	}
}
