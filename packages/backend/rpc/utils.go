package rpc

//import (
//	"strings"
//	"unicode"
//)
//
//func FormatFilename(filename string) string {
//	formated := addSpaceBetweenCase(filename)
//	formated = strings.ReplaceAll(formated, "_", " ")
//	formated = strings.ReplaceAll(formated, "-", " ")
//	formated = strings.ReplaceAll(formated, ".", " ")
//	formated = strings.ReplaceAll(formated, "/", " ")
//	return formated
//}
//
//func addSpaceBetweenCase(str string) string {
//	var result string
//	for i, r := range str {
//		if i > 0 {
//			last := rune(str[i-1])
//			if unicode.IsLower(last) && unicode.IsUpper(r) {
//				result += " "
//			}
//		}
//		result += string(r)
//	}
//	return result
//}
