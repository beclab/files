// 包名：gosearpc
package gosearpc

import (
	"fmt"
	"strings"
	"text/template"
)

// 定义模块方法数组模板
var moduleFuncArrayTemplate = `
static PyMethodDef SearpcClientModule_Functions[] = {
{{.Array}}
    {NULL, NULL, 0, NULL},
};
`

// 定义单个函数项模板
var funcItemTemplate = `
    {"{{.Pyfuncname}}", (PyCFunction){{.Cfuncname}},
     METH_VARARGS, "" },`

// 类型转换表（假设已在其他文件定义）
var typeTable = map[string]struct {
	CType string
	Fmt   string
}{
	"string": {"char *", "z"},
	"int":    {"int", "i"},
	"int64":  {"gint64", "L"},
	"object": {"GObject *", "O"},
}

// 生成fcall方法数组项
func genFcallFuncsArray(argTypes []string) string {
	var pyfuncname, cfuncname string

	if len(argTypes) == 0 {
		pyfuncname = "fcall__void"
		cfuncname = "SearpcClient_Fcall__Void"
	} else {
		pyfuncname = "fcall__" + strings.Join(argTypes, "_")
		var tmplist []string
		for _, arg := range argTypes {
			tmplist = append(tmplist, strings.Title(arg))
		}
		cfuncname = "SearpcClient_Fcall__" + strings.Join(tmplist, "_")
	}

	// 使用模板生成函数项
	tmpl, _ := template.New("funcItem").Parse(funcItemTemplate)
	var buf strings.Builder
	tmpl.Execute(&buf, struct {
		Pyfuncname string
		Cfuncname  string
	}{
		Pyfuncname: pyfuncname,
		Cfuncname:  cfuncname,
	})
	return buf.String()
}

// 生成fret方法数组项
func genFretFuncsArray(retType string) string {
	var pyfuncname, cfuncname string

	if retType == "" {
		pyfuncname = "fret__void"
		cfuncname = "SearpcClient_Fret__Void"
	} else {
		pyfuncname = "fret__" + retType
		cfuncname = "SearpcClient_Fret__" + strings.Title(retType)
	}

	// 使用模板生成函数项
	tmpl, _ := template.New("funcItem").Parse(funcItemTemplate)
	var buf strings.Builder
	tmpl.Execute(&buf, struct {
		Pyfuncname string
		Cfuncname  string
	}{
		Pyfuncname: pyfuncname,
		Cfuncname:  cfuncname,
	})
	return buf.String()
}

// 生成模块方法数组
func GenModuleFuncsArray() string {
	// 假设func_table已在其他文件定义
	var funcTable = []struct {
		Name     string
		ArgTypes []string
	}{
		// 示例数据，实际应从rpc_table导入
		{"func1", []string{"int", "string"}},
		{"func2", []string{"string"}},
	}

	// 收集所有参数类型组合
	argTypesList := make(map[string]bool)
	for _, item := range funcTable {
		key := strings.Join(item.ArgTypes, ",")
		argTypesList[key] = true
	}

	// 生成fcall方法数组
	var fcallArray strings.Builder
	for args := range argTypesList {
		argTypes := strings.Split(args, ",")
		fcallArray.WriteString(genFcallFuncsArray(argTypes))
	}

	// 生成fret方法数组（示例类型）
	var fretArray strings.Builder
	retTypes := []string{"int", "int64", "string"}
	for _, retType := range retTypes {
		fretArray.WriteString(genFretFuncsArray(retType))
	}

	// 组合最终数组
	array := fcallArray.String() + fretArray.String()

	// 使用模板生成最终输出
	tmpl, _ := template.New("moduleFunc").Parse(moduleFuncArrayTemplate)
	var buf strings.Builder
	tmpl.Execute(&buf, struct{ Array string }{Array: array})
	return buf.String()
}

// fcall函数模板
var fcallTemplate = `
static PyObject *
SearpcClient_Fcall__{{.Suffix}}(PyObject *self,
                              PyObject *args)
{
    char *fname;
{{.DefArgs}}
    char *fcall;
    gsize len;

    if (!PyArg_ParseTuple(args, "{{.Fmt}}", {{.ArgsAddr}}))
        return NULL;

    fcall = searpc_client_fcall__{{.Suffix}}({{.Args}});

    return PyString_FromString(fcall);
}
`

// 生成fcall函数实现
func genFcallFunc(argTypes []string) string {
	var suffix, Suffix, defArgs, argsAddr, args, format string

	if len(argTypes) == 0 {
		Suffix = "Void"
		suffix = "void"
		defArgs = ""
		argsAddr = "&fname"
		args = "fname, &len"
		format = "s"
	} else {
		var tmplist []string
		for _, arg := range argTypes {
			tmplist = append(tmplist, strings.Title(arg))
		}
		Suffix = strings.Join(tmplist, "_")
		suffix = strings.Join(argTypes, "_")

		// 构建参数定义部分
		var defArgsBuilder strings.Builder
		var argsAddrBuilder strings.Builder
		var argsBuilder strings.Builder
		fmtBuilder := strings.Builder{}
		fmtBuilder.WriteString("s")

		for i, argType := range argTypes {
			defArgsBuilder.WriteString(fmt.Sprintf("    %s param%d;\n", typeTable[argType].CType, i+1))
			argsAddrBuilder.WriteString(fmt.Sprintf(", &param%d", i+1))
			argsBuilder.WriteString(fmt.Sprintf(", param%d", i+1))
			fmtBuilder.WriteString(typeTable[argType].Fmt)
		}

		defArgs = defArgsBuilder.String()
		argsAddr = "&fname" + argsAddrBuilder.String()
		args = "fname" + argsBuilder.String() + ", &len"
		format = fmtBuilder.String()
	}

	// 使用模板生成函数
	tmpl, _ := template.New("fcallFunc").Parse(fcallTemplate)
	var buf strings.Builder
	tmpl.Execute(&buf, struct {
		Suffix   string
		suffix   string
		DefArgs  string
		ArgsAddr string
		Args     string
		Fmt      string
	}{
		Suffix:   Suffix,
		suffix:   suffix,
		DefArgs:  defArgs,
		ArgsAddr: argsAddr,
		Args:     args,
		Fmt:      format,
	})
	return buf.String()
}

// 生成所有fcall函数列表
func GenFcallList() string {
	// 假设func_table已在其他文件定义
	var funcTable = []struct {
		Name     string
		ArgTypes []string
	}{
		// 示例数据，实际应从rpc_table导入
		{"func1", []string{"int", "string"}},
		{"func2", []string{"string"}},
	}

	// 收集所有参数类型组合
	argTypesList := make(map[string]bool)
	for _, item := range funcTable {
		key := strings.Join(item.ArgTypes, ",")
		argTypesList[key] = true
	}

	// 生成所有fcall函数
	var output strings.Builder
	for args := range argTypesList {
		argTypes := strings.Split(args, ",")
		output.WriteString(genFcallFunc(argTypes))
	}
	return output.String()
}

// 主函数（示例）
func main() {
	fmt.Println(GenFcallList())
	fmt.Println(GenModuleFuncsArray())
}
