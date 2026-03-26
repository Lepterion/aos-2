package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// Констатни формату за варіантом
const (
	ExpBits = 9
	ManBits = 32
	Bias    = (1 << (ExpBits - 1)) - 1 // Для 9 біт зміщення = 255
)

// Float42 - наш власний 42-бітний тип (зберігається у 64-бітній цілочисельній змінній)
type Float42 uint64

// Конвертація між нашим типом та нативним

func Float64ToFloat42(f float64) Float42 {
	bits := math.Float64bits(f)
	sign := (bits >> 63) & 1

	if math.IsNaN(f) {
		exp42 := uint64((1 << ExpBits) - 1)
		man42 := uint64(1)
		return Float42((sign << (ExpBits + ManBits)) | (exp42 << ManBits) | man42)
	}

	if math.IsInf(f, 0) {
		exp42 := uint64((1 << ExpBits) - 1)
		return Float42((sign << (ExpBits + ManBits)) | (exp42 << ManBits))
	}

	if f == 0.0 {
		return Float42(sign << (ExpBits + ManBits))
	}

	exp64 := int((bits >> 52) & 0x7FF)
	man64 := bits & 0xFFFFFFFFFFFFF

	exp42 := exp64 - 1023 + Bias
	var man42 uint64

	if exp42 <= 0 {
		exp42 = 0
		shift := 1 - exp42
		if shift < 53 {
			man64 = (man64 | (1 << 52)) >> shift
			man42 = man64 >> (52 - ManBits)
		} else {
			man42 = 0
		}
	} else if exp42 >= (1<<ExpBits)-1 {
		exp42 = (1 << ExpBits) - 1
		man42 = 0
	} else {
		man42 = man64 >> (52 - ManBits)
	}

	return Float42((sign << (ExpBits + ManBits)) | (uint64(exp42) << ManBits) | man42)
}

func Float42ToFloat64(f Float42) float64 {
	sign := (uint64(f) >> (ExpBits + ManBits)) & 1
	exp42 := int((uint64(f) >> ManBits) & ((1 << ExpBits) - 1))
	man42 := uint64(f) & ((1 << ManBits) - 1)

	if exp42 == (1<<ExpBits)-1 {
		if man42 == 0 {
			if sign == 1 {
				return math.Inf(-1)
			}
			return math.Inf(1)
		}
		return math.NaN()
	}

	if exp42 == 0 {
		if man42 == 0 {
			return math.Float64frombits(sign << 63)
		}
		return math.Float64frombits((sign << 63) | (uint64(0) << 52) | (man42 << (52 - ManBits)))
	}

	exp64 := exp42 - Bias + 1023
	man64 := man42 << (52 - ManBits)
	return math.Float64frombits((sign << 63) | (uint64(exp64) << 52) | man64)
}

func PrintBits(f Float42) string {
	sign := (uint64(f) >> (ExpBits + ManBits)) & 1
	exp := (uint64(f) >> ManBits) & ((1 << ExpBits) - 1)
	man := uint64(f) & ((1 << ManBits) - 1)

	implicit := 1
	typeStr := "Нормалізоване"

	if exp == (1<<ExpBits)-1 {
		implicit = 0
		if man == 0 {
			typeStr = "Infinity"
		} else {
			typeStr = "NaN"
		}
	} else if exp == 0 {
		implicit = 0
		if man == 0 {
			typeStr = "Нуль"
		} else {
			typeStr = "Ненормалізоване"
		}
	}

	return fmt.Sprintf("[%s] Знак: %b | Експ: %09b | Неявн: %b | Мант: %032b (val: %f)",
		typeStr, sign, exp, implicit, man, Float42ToFloat64(f))
}

// СОПроцессор

type StackNode struct {
	val  Float42
	next *StackNode
}

type Coprocessor struct {
	head  *StackNode
	count int
	vars  map[string]float64
}

func NewCoprocessor() *Coprocessor {
	return &Coprocessor{
		vars: make(map[string]float64),
	}
}

func (c *Coprocessor) SetVar(name string, val float64) {
	c.vars[name] = val
}

func (c *Coprocessor) PrintState(instruction string) {
	fmt.Printf("\n--- Виконано: %s ---\n", instruction)
	if c.head == nil {
		fmt.Println("Стек порожній")
	} else {
		curr := c.head
		i := 0
		for curr != nil {
			fmt.Printf("Регістр %d: %s\n", i, PrintBits(curr.val))
			curr = curr.next
			i++
		}
	}
	fmt.Print("\n[Натисни Enter для продовження]")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

// Інструкції для інтерпретації

// --- ВНУТРІШНІ ОПЕРАЦІЇ СТЕКУ ---

func (c *Coprocessor) PUSH(val float64) {
	if c.count >= 8 {
		panic("Переповнення стеку! (Максимум 8 регістрів)")
	}
	newNode := &StackNode{val: Float64ToFloat42(val), next: c.head}
	c.head = newNode
	c.count++
}

func (c *Coprocessor) POP() float64 {
	if c.head == nil {
		panic("Стек порожній!")
	}
	val := c.head.val
	c.head = c.head.next
	c.count--
	return Float42ToFloat64(val)
}

// --- ІНСТРУКЦІЇ СОПРОЦЕСОРА ---

// LOADC - завантажує числову константу
func (c *Coprocessor) LOADC(val float64) {
	c.PUSH(val)
	c.PrintState(fmt.Sprintf("LOADC %f", val))
}

// LOADV - завантажує значення зі змінної
func (c *Coprocessor) LOADV(name string) {
	if val, ok := c.vars[name]; ok {
		c.PUSH(val)
		c.PrintState(fmt.Sprintf("LOADV %s (val: %f)", name, val))
	} else {
		panic(fmt.Sprintf("Змінна '%s' не знайдена!", name))
	}
}

// STORE - витягує зі стеку і зберігає у змінну
func (c *Coprocessor) STORE(name string) {
	val := c.POP()
	c.vars[name] = val
	c.PrintState(fmt.Sprintf("STORE %s (збережено: %f)", name, val))
}

func (c *Coprocessor) ADD() {
	b := c.POP()
	a := c.POP()
	c.PUSH(a + b)
	c.PrintState("ADD")
}

func (c *Coprocessor) SUB() {
	b := c.POP() // Від'ємник
	a := c.POP() // Зменшуване
	c.PUSH(a - b)
	c.PrintState("SUB")
}

func (c *Coprocessor) MULT() {
	b := c.POP()
	a := c.POP()
	c.PUSH(a * b)
	c.PrintState("MULT")
}

func (c *Coprocessor) DIV() {
	b := c.POP()
	a := c.POP()
	c.PUSH(a / b)
	c.PrintState("DIV")
}

func (c *Coprocessor) DUP() {
	if c.head == nil {
		panic("Стек порожній, неможливо дублювати!")
	}
	val := Float42ToFloat64(c.head.val)
	c.PUSH(val)
	c.PrintState("DUP")
}

func (c *Coprocessor) SWAP() {
	b := c.POP()
	a := c.POP()
	c.PUSH(b)
	c.PUSH(a)
	c.PrintState("SWAP")
}

// Інтерпретатор коду з файлу

func (c *Coprocessor) ExecuteFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("не вдалося відкрити файл %s: %v", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			lineNum++
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToUpper(parts[0])

		switch cmd {
		case "LOADC":
			if len(parts) < 2 {
				return fmt.Errorf("помилка у рядку %d: LOADC потребує числового аргументу", lineNum)
			}
			arg := strings.Replace(parts[1], ",", ".", 1)
			val, err := strconv.ParseFloat(arg, 64)
			if err != nil {
				return fmt.Errorf("помилка у рядку %d: LOADC приймає тільки числа. Для змінних використовуйте LOADV", lineNum)
			}
			c.LOADC(val)

		case "LOADV":
			if len(parts) < 2 {
				return fmt.Errorf("помилка у рядку %d: LOADV потребує імені змінної", lineNum)
			}
			c.LOADV(parts[1])

		case "STORE":
			if len(parts) < 2 {
				return fmt.Errorf("помилка у рядку %d: STORE потребує імені змінної куди зберегти результат", lineNum)
			}
			c.STORE(parts[1])

		case "ADD":
			c.ADD()
		case "SUB":
			c.SUB()
		case "MULT":
			c.MULT()
		case "DIV":
			c.DIV()
		case "DUP":
			c.DUP()
		case "SWAP":
			c.SWAP()
		default:
			return fmt.Errorf("помилка у рядку %d: невідома команда '%s'", lineNum, cmd)
		}
		lineNum++
	}

	return scanner.Err()
}

// Entrypoint

func main() {
	fmt.Println("=== Стандартні представлення ЧПТ (Варіант: p=9, q=32) ===")
	fmt.Println(PrintBits(Float64ToFloat42(0.0)))
	fmt.Println(PrintBits(Float64ToFloat42(math.Inf(1))))
	fmt.Println(PrintBits(Float64ToFloat42(math.Inf(-1))))
	fmt.Println(PrintBits(Float64ToFloat42(math.NaN())))
	fmt.Println(PrintBits(Float64ToFloat42(1.0)))
	fmt.Println("=========================================================\n")

	cpu := NewCoprocessor()

	var a, b float64
	fmt.Print("Введіть значення для змінної a: ")
	fmt.Scanln(&a)
	fmt.Print("Введіть значення для змінної b: ")
	fmt.Scanln(&b)

	cpu.SetVar("a", a)
	cpu.SetVar("b", b)

	fmt.Println("\nВиконання команд з файлу code.txt...")

	err := cpu.ExecuteFromFile("code.txt")
	if err != nil {
		fmt.Printf("Помилка виконання: %v\n", err)
		return
	}

	if val, ok := cpu.vars["result"]; ok {
		fmt.Printf("\nФінальний результат обчислень (змінна result): %f\n", val)
	}

	fmt.Println("\nПрограму успішно завершено!")
}
