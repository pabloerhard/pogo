package virtualmachine

import (
	"fmt"
	"pogo/src/shared"
	"strings"
)

type VirtualMachine struct {
	quads              []shared.Quadruple
	memoryManager      *MemoryManager
	instructionPointer int
	returnPointer      int
}

func NewVirtualMachine(quads []shared.Quadruple, memManager *MemoryManager) *VirtualMachine {
	return &VirtualMachine{
		quads:              quads,
		memoryManager:      memManager,
		instructionPointer: 1,
		returnPointer:      -1,
	}
}

func (vm *VirtualMachine) Execute() error {
	for vm.instructionPointer < len(vm.quads) {
		quad := vm.quads[vm.instructionPointer]

		if err := vm.executeQuadruple(quad); err != nil {
			return fmt.Errorf("error at instruction %d: %v", vm.instructionPointer, err)
		}

		vm.instructionPointer++
	}
	return nil
}

func (vm *VirtualMachine) executeQuadruple(quad shared.Quadruple) error {
	switch quad.Operator {
	case "+", "-", "*", "/":
		return vm.executeArithmetic(quad)
	case "=":
		return vm.executeAssignment(quad)
	case "<", ">", "==", "!=":
		return vm.executeComparison(quad)
	case "print":
		return vm.executePrint(quad)
	case "goto":
		return vm.executeGoto(quad)
	case "gotof":
		return vm.executeGotof(quad)
	}

	return nil
}

func (vm *VirtualMachine) executeArithmetic(quad shared.Quadruple) error {
	leftVal, err := vm.memoryManager.Load(quad.LeftOp.(int))

	if err != nil {
		return fmt.Errorf("failed to load left operand: %v", err)
	}

	rightVal, err := vm.memoryManager.Load(quad.RightOp.(int))
	if err != nil {
		return fmt.Errorf("failed to load right operand: %v", err)
	}

	var result interface{}

	switch left := leftVal.(type) {
	case int:
		right, ok := rightVal.(int)
		if !ok {
			return fmt.Errorf("type mismatch: cannot perform integer operation with %T", rightVal)
		}

		switch quad.Operator {
		case "+":
			result = left + right
		case "-":
			result = left - right
		case "*":
			result = left * right
		case "/":
			if right == 0 {
				return fmt.Errorf("division by zero")
			}
			result = left / right
		}

	case float64:
		var right float64
		switch r := rightVal.(type) {
		case float64:
			right = r
		case int:
			right = float64(r)
		default:
			return fmt.Errorf("type mismatch: cannot perform float operation with %T", rightVal)
		}

		switch quad.Operator {
		case "+":
			result = left + right
		case "-":
			result = left - right
		case "*":
			result = left * right
		case "/":
			if right == 0 {
				return fmt.Errorf("division by zero")
			}
			result = left / right
		}
	}

	// Store result in memory
	return vm.memoryManager.Store(quad.Result.(int), result)
}

func (vm *VirtualMachine) executeAssignment(quad shared.Quadruple) error {
	value, err := vm.memoryManager.Load(quad.LeftOp.(int))
	if err != nil {
		return fmt.Errorf("failed to load source value: %v", err)
	}
	return vm.memoryManager.Store(quad.Result.(int), value)
}

func (vm *VirtualMachine) executeComparison(quad shared.Quadruple) error {
	// fmt.Println("Entering execution")
	leftVal, err := vm.memoryManager.Load(quad.LeftOp.(int))
	fmt.Println("This is the leftVal", leftVal)
	if err != nil {
		return fmt.Errorf("failed to load left operand: %v", err)
	}

	rightVal, err := vm.memoryManager.Load(quad.RightOp.(int))
	if err != nil {
		return fmt.Errorf("failed to load right operand: %v", err)
	}
	// fmt.Println("This is the rightVal", rightVal)

	var leftFloat, rightFloat float64

	switch v := leftVal.(type) {
	case int:
		leftFloat = float64(v)
	case float64:
		leftFloat = v
	default:
		return fmt.Errorf("invalid type for comparison: %T", leftVal)
	}

	switch v := rightVal.(type) {
	case int:
		rightFloat = float64(v)
	case float64:
		rightFloat = v
	default:
		return fmt.Errorf("invalid type for comparison: %T", rightVal)
	}

	var result bool
	var intResult int

	switch quad.Operator {
	case "<":
		result = leftFloat < rightFloat
	case ">":
		result = leftFloat > rightFloat
	case "==":
		result = leftFloat == rightFloat
	case "!=":
		result = leftFloat != rightFloat
	}

	if result {
		intResult = 1
	} else {
		intResult = 0
	}
	// fmt.Println("This is where we store", quad.Result)
	return vm.memoryManager.Store(quad.Result.(int), intResult)
}

func (vm *VirtualMachine) executePrint(quad shared.Quadruple) error {
	value, err := vm.memoryManager.Load(quad.LeftOp.(int))
	if err != nil {
		return fmt.Errorf("failed to load print value: %v", err)
	}

	switch v := value.(type) {
	case string:
		cleanStr := strings.Trim(v, "\"")
		fmt.Println(cleanStr)
	case int:
		fmt.Println(v)
	case float64:
		fmt.Println(v)
	default:
		return fmt.Errorf("unsupported type for printing: %T", value)
	}

	return nil
}

func (vm *VirtualMachine) executeGoto(quad shared.Quadruple) error {
	vm.instructionPointer = quad.Result.(int)
	return nil
}

func (vm *VirtualMachine) executeGotof(quad shared.Quadruple) error {
	condValue, err := vm.memoryManager.Load(quad.LeftOp.(int))
	if err != nil {
		return err
	}

	if condValue == 0 {
		vm.instructionPointer = quad.Result.(int) - 1
	}

	return nil
}
