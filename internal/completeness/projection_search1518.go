package completeness

import (
	"errors"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// The ACP full response retains a generic result; the ordinary response uses
// its search payload and adds the web search type. Only the captured, completed
// query-only Gemini search has been qualified.
func projectSearchWeb1518(step, generic, result, payload protoreflect.Message) (proto.Message, error) {
	unsupported := errors.New("unsupported search_web projection")
	query, ok := genericArg(generic, "query")
	if genericToolName(step) != "search_web" || statusName(step) != "CORTEX_STEP_STATUS_DONE" ||
		!onlyFields(step, fieldSet("type", "status", "metadata", "generic")) ||
		!onlyFields(result, fieldSet("result", "payload", "full_output_uri", "step_render_info")) ||
		!ok || query == "" || !genericArgsAllowed(generic, "query") || !argumentsPresented(generic, step) ||
		!strings.HasPrefix(tool143Get(result, "full_output_uri").String(), "file:///") {
		return nil, unsupported
	}
	decoded, err := decode("exa.cortex.core.tools.SearchWeb", tool143Get(payload, "value").Bytes())
	if err != nil {
		return nil, err
	}
	search := decoded.ProtoReflect()
	summary := tool143Get(search, "summary").String()
	provider := tool143Get(search, "third_party_config").Message()
	documents := tool143Get(search, "web_documents").List()
	if !onlyFields(search, fieldSet("query", "web_documents", "summary", "third_party_config")) ||
		tool143Get(search, "query").String() != query || summary == "" || documents.Len() == 0 ||
		enumName(provider, provider.Descriptor().Fields().ByName("provider")) != "THIRD_PARTY_WEB_SEARCH_PROVIDER_GEMINI" {
		return nil, unsupported
	}
	for i := 0; i < documents.Len(); i++ {
		document := documents.Get(i).Message()
		if !onlyFields(document, fieldSet("document_id", "url", "title")) ||
			tool143Get(document, "document_id").String() == "" || tool143Get(document, "url").String() == "" ||
			tool143Get(document, "title").String() == "" {
			return nil, unsupported
		}
	}
	render := tool143Get(result, "step_render_info").Message()
	parts := tool143Get(render, "title_parts").List()
	body, timed := timestampedBody159(tool143Get(result, "result").String())
	if !timed || body != "The search for \""+query+"\" returned the following summary:\n"+summary+
		"\n\nNote: Source URLs are HTTP redirects; check where they resolve if you need the direct link." ||
		!onlyFields(render, fieldSet("title_prefix", "icon", "aggregation_key", "title_parts")) ||
		tool143Get(render, "title_prefix").String() != "Searched" || tool143Get(render, "icon").String() != "search" ||
		tool143Get(render, "aggregation_key").String() != "web" || parts.Len() != 1 ||
		!onlyFields(parts.Get(0).Message(), fieldSet("text")) || tool143Get(parts.Get(0).Message(), "text").String() != query {
		return nil, unsupported
	}

	out := proto.Clone(step.Interface())
	native := tool143Native(out.ProtoReflect(), "SEARCH_WEB", "search_web")
	tool143Copy(native, search, "query", "web_documents", "summary", "third_party_config")
	kind := native.Descriptor().Fields().ByName("search_type")
	native.Set(kind, protoreflect.ValueOfEnum(kind.Enum().Values().ByName("SEARCH_WEB_TYPE_WEB").Number()))
	return out, nil
}
