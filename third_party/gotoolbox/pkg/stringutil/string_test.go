package stringutil_test

import (
	"strings"

	. "github.com/smartystreets/goconvey/convey"

	"testing"

	"github.com/wangweihong/gotoolbox/pkg/stringutil"
)

func TestBothEmptyOrNone(t *testing.T) {
	Convey("BothEmptyOrNone", t, func() {
		So(stringutil.BothEmptyOrNone("a", ""), ShouldBeFalse)
		So(stringutil.BothEmptyOrNone("a", "b"), ShouldBeTrue)
		So(stringutil.BothEmptyOrNone("", "b"), ShouldBeFalse)
	})
}

func TestHasAnyPrefix(t *testing.T) {
	Convey("HasAnyPrefix", t, func() {
		str := "tcp://192.168.134.132"
		So(stringutil.HasAnyPrefix(str, ""), ShouldBeFalse)
		So(stringutil.HasAnyPrefix("", ""), ShouldBeFalse)
		So(stringutil.HasAnyPrefix(str, "http", "https"), ShouldBeFalse)
		So(stringutil.HasAnyPrefix(str, "http", ""), ShouldBeFalse)
		So(stringutil.HasAnyPrefix(str, "tcp", "unix"), ShouldBeTrue)
	})
}

func TestPointerToString(t *testing.T) {
	Convey("ToString", t, func() {
		s := "a"
		var sp *string
		So(stringutil.PointerToString(sp), ShouldEqual, "")
		sp = &s
		So(stringutil.PointerToString(sp), ShouldEqual, "a")
	})
}

func TestAddIf(t *testing.T) {
	Convey("AddIf", t, func() {
		a := "str"
		So(stringutil.AddPrefixIfNotHas(a, "my"), ShouldEqual, "mystr")
		So(stringutil.AddSuffixIfNotHas(a, "my"), ShouldEqual, "strmy")

		b := "prefixmysuffix"
		So(stringutil.AddSuffixIfNotHas(b, "suffix"), ShouldEqual, "prefixmysuffix")
		So(stringutil.AddPrefixIfNotHas(b, "prefix"), ShouldEqual, "prefixmysuffix")
	})
}

func TestRemoveBeforeStr(t *testing.T) {
	Convey("RemoveSub*", t, func() {
		Convey("RemoveSubBefore", func() {
			a := "AAAstrBBB"
			So(stringutil.RemoveSubBefore(a, "str"), ShouldEqual, "strBBB")
			So(stringutil.RemoveSubBefore("CCC", "str"), ShouldEqual, "CCC")
		})

		Convey("RemoveSubAndBefore", func() {
			a := "AAAstrBBB"
			So(stringutil.RemoveSubAndBefore(a, "str"), ShouldEqual, "BBB")
			So(stringutil.RemoveSubBefore("CCC", "str"), ShouldEqual, "CCC")
		})
	})
}

func TestTrimAnyPrefix(t *testing.T) {
	Convey("TestTrimAnyPrefix*", t, func() {
		Convey("TestTrimAnyPrefix", func() {
			a := "https://127.0.0.1"
			So(stringutil.TrimAnyPrefix(a, "http://", "https://"), ShouldEqual, "127.0.0.1")
		})
	})
}

func TestTrimAnyPrefixAndReturn(t *testing.T) {
	Convey("TestTrimAnyPrefix*", t, func() {
		Convey("TestTrimAnyPrefix", func() {
			a := "https://127.0.0.1"
			trims, d := stringutil.TrimAnyPrefixAndReturn(a, "http://", "https://")
			So(d, ShouldEqual, "127.0.0.1")
			So(trims, ShouldResemble, []string{"https://"})
		})
	})
}

func TestExtrace(t *testing.T) {
	Convey("TestExtrace ", t, func() {
		words := []string{"RTX", "L", "H", "A", "T", "P", "M"}
		input := "RTXLHMD"
		So(stringutil.ExtractTokens(words, input), ShouldResemble, []string{"RTX", "L", "H", "M"})
		So(stringutil.ExtractTokens([]string{"RGX", "L", "H", "A", "T", "P", "M"}, input), ShouldResemble, []string{"T", "L", "H", "M"})
		So(stringutil.ExtractTokens([]string{"P", "M"}, input), ShouldResemble, []string{"M"})
	})
}

func TestField(t *testing.T) {
	Convey("TestField", t, func() {
		data := "swift deploy --model deepseek-ai/DeepSeek-R1-Distill-Qwen-14B --model_type deepseek_r1_distill --template deepseek_r1 --infer_backend vllm --max_new_tokens 2048 --top_k 20 --top_p 0.700000 --repetition_penalty 1.050000 --gpu_memory_utilization 0.900000 --tensor_parallel_size 1 --pipeline_parallel_size 1 --max_num_seqs 256 --max_model_len 12000 --served_model_name DeepSeek --max_shard_size 40G"
		sf := strings.Fields(data)
		t.Log(sf)
	})
}

func TestShellParse(t *testing.T) {
	rawCmd := `swift deploy '--host' 0.0.0.0 '--port' '8000' '--model' deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B '--model_type' \
	deepseek_r1_distill \
	'--template' \
	deepseek_r1 \
	'--system' \
	"xxx" \
	'--infer_backend' \
	pt \
	'--max_new_tokens' \
	'2048' \
	'--top_k' \
	'20' \
	'--top_p' \
	'0.700000' \
	'--repetition_penalty' \
	'1.050000'`

	Convey("TestShellParse", t, func() {
		Convey("long", func() {
			ret := "swift deploy --host 0.0.0.0 --port 8000 --model deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B --model_type deepseek_r1_distill --template deepseek_r1 --system xxx --infer_backend pt --max_new_tokens 2048 --top_k 20 --top_p 0.700000 --repetition_penalty 1.050000"
			parsedRet, err := stringutil.ShellParse2(rawCmd)
			So(err, ShouldBeNil)
			So(parsedRet, ShouldEqual, ret)
		})
		Convey("hasquote", func() {
			ret := "bash -c 'sleep 10000'"
			parsedRet, err := stringutil.ShellParse(ret)
			So(err, ShouldBeNil)
			So(len(parsedRet), ShouldEqual, 3)
			So(parsedRet[2], ShouldEqual, "sleep 10000")
			for _, v := range parsedRet {
				t.Log(v)
			}
		})
	})

}
