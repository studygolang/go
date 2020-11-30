// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unicode_test

import (
	"fmt"
	"unicode"
)

// 以 Is 开头的函数可用于检查码点属于哪个范围表。注意，码点可能符合多个范围。
func Example_is() {

	// 带又混合类型码点的常量。
	const mixed = "\b5Ὂg̀9! ℃ᾭG"
	for _, c := range mixed {
		fmt.Printf("For %q:\n", c)
		if unicode.IsControl(c) {
			fmt.Println("\tis control rune")
		}
		if unicode.IsDigit(c) {
			fmt.Println("\tis digit rune")
		}
		if unicode.IsGraphic(c) {
			fmt.Println("\tis graphic rune")
		}
		if unicode.IsLetter(c) {
			fmt.Println("\tis letter rune")
		}
		if unicode.IsLower(c) {
			fmt.Println("\tis lower case rune")
		}
		if unicode.IsMark(c) {
			fmt.Println("\tis mark rune")
		}
		if unicode.IsNumber(c) {
			fmt.Println("\tis number rune")
		}
		if unicode.IsPrint(c) {
			fmt.Println("\tis printable rune")
		}
		if !unicode.IsPrint(c) {
			fmt.Println("\tis not printable rune")
		}
		if unicode.IsPunct(c) {
			fmt.Println("\tis punct rune")
		}
		if unicode.IsSpace(c) {
			fmt.Println("\tis space rune")
		}
		if unicode.IsSymbol(c) {
			fmt.Println("\tis symbol rune")
		}
		if unicode.IsTitle(c) {
			fmt.Println("\tis title case rune")
		}
		if unicode.IsUpper(c) {
			fmt.Println("\tis upper case rune")
		}
	}

	// 输出：
	// For '\b':
	// 	控制码点
	// 	不可打印码点
	// For '5':
	// 	数字码点
	// 	图形码点
	// 	数字码点
	// 	可打印码点
	// For 'Ὂ':
	// 	图形码点
	// 	字符码点
	// 	可打印码点
	// 	大写码点
	// For 'g':
	// 	图形码点
	// 	字母码点
	// 	小写码点
	// 	可打印码点
	// For '̀':
	// 	图形码点
	// 	mark 码点
	// 	可打印码点
	// For '9':
	// 	十进制码点
	// 	图形码点
	// 	数字码点
	// 	可打印码点
	// For '!':
	// 	图形码点
	// 	可打印码点
	// 	标点符号点
	// For ' ':
	// 	图形码点
	// 	可打印码点
	// 	空格码点
	// For '℃':
	// 	图形码点
	// 	可打印码点
	// 	符号码点
	// For 'ᾭ':
	// 	图形码点
	// 	字母码点
	// 	可打印码点
	// 	首字母大写码点
	// For 'G':
	// 	图形码点
	// 	字母码点
	// 	可打印码点
	// 	大写码点
}

func ExampleSimpleFold() {
	fmt.Printf("%#U\n", unicode.SimpleFold('A'))      // 'a'
	fmt.Printf("%#U\n", unicode.SimpleFold('a'))      // 'A'
	fmt.Printf("%#U\n", unicode.SimpleFold('K'))      // 'k'
	fmt.Printf("%#U\n", unicode.SimpleFold('k'))      // '\u212A' (Kelvin symbol, K)
	fmt.Printf("%#U\n", unicode.SimpleFold('\u212A')) // 'K'
	fmt.Printf("%#U\n", unicode.SimpleFold('1'))      // '1'

	// 输出：
	// U+0061 'a'
	// U+0041 'A'
	// U+006B 'k'
	// U+212A 'K'
	// U+004B 'K'
	// U+0031 '1'
}

func ExampleTo() {
	const lcG = 'g'
	fmt.Printf("%#U\n", unicode.To(unicode.UpperCase, lcG))
	fmt.Printf("%#U\n", unicode.To(unicode.LowerCase, lcG))
	fmt.Printf("%#U\n", unicode.To(unicode.TitleCase, lcG))

	const ucG = 'G'
	fmt.Printf("%#U\n", unicode.To(unicode.UpperCase, ucG))
	fmt.Printf("%#U\n", unicode.To(unicode.LowerCase, ucG))
	fmt.Printf("%#U\n", unicode.To(unicode.TitleCase, ucG))

	// 输出：
	// U+0047 'G'
	// U+0067 'g'
	// U+0047 'G'
	// U+0047 'G'
	// U+0067 'g'
	// U+0047 'G'
}

func ExampleToLower() {
	const ucG = 'G'
	fmt.Printf("%#U\n", unicode.ToLower(ucG))

	// 输出：
	// U+0067 'g'
}
func ExampleToTitle() {
	const ucG = 'g'
	fmt.Printf("%#U\n", unicode.ToTitle(ucG))

	// 输出：
	// U+0047 'G'
}

func ExampleToUpper() {
	const ucG = 'g'
	fmt.Printf("%#U\n", unicode.ToUpper(ucG))

	// 输出：
	// U+0047 'G'
}

func ExampleSpecialCase() {
	t := unicode.TurkishCase

	const lci = 'i'
	fmt.Printf("%#U\n", t.ToLower(lci))
	fmt.Printf("%#U\n", t.ToTitle(lci))
	fmt.Printf("%#U\n", t.ToUpper(lci))

	const uci = 'İ'
	fmt.Printf("%#U\n", t.ToLower(uci))
	fmt.Printf("%#U\n", t.ToTitle(uci))
	fmt.Printf("%#U\n", t.ToUpper(uci))

	// 输出：
	// U+0069 'i'
	// U+0130 'İ'
	// U+0130 'İ'
	// U+0069 'i'
	// U+0130 'İ'
	// U+0130 'İ'
}
