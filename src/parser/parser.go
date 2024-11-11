package parser

import (
	"fmt"
	"pogo/src/lexer"
	"pogo/src/semantic"
	"pogo/src/shared"
	"pogo/src/token"
)

type Parser struct {
	lexer         *lexer.Lexer
	curr          *token.Token
	symbolTable   *semantic.SymbolTable
	codeGenerator *semantic.QuadrupleList
}

func NewParser(l *lexer.Lexer) *Parser {
	p := &Parser{
		lexer:         l,
		symbolTable:   semantic.NewSymbolTable(),
		codeGenerator: semantic.NewQuadrupleList(),
	}
	p.next() // prime first token
	return p
}

func (p *Parser) ParseProgram() error {
	p.codeGenerator.HandleProgramStart()

	if err := p.parseProgramName(); err != nil {
		return err
	}

	if err := p.parseVarDeclarationSection(false); err != nil {
		return err
	}

	if err := p.parseFunctionListOpt(); err != nil {
		return err
	}

	if err := p.parseMainSection(); err != nil {
		return err
	}
	p.codeGenerator.Print()
	p.codeGenerator.PrintStacks()

	return nil
}

func (p *Parser) parseProgramName() error {
	if err := p.expect(token.TokMap.Type("kwdProgram")); err != nil {
		return err
	}
	if err := p.expect(token.TokMap.Type("id")); err != nil {
		return err
	}
	return p.expect(token.TokMap.Type("terminator"))
}

func (p *Parser) parseVarDeclarationSection(isFunction bool) error {
	if p.curr.Type != token.TokMap.Type("kwdVars") {
		if p.curr.Type != token.TokMap.Type("kwdFunc") && p.curr.Type != token.TokMap.Type("kwdBegin") && !isFunction {
			return fmt.Errorf("line %d: unexpected token '%s', expected 'var', 'func', or 'begin'", p.curr.Line, p.curr.Lit)
		}
		return nil
	}

	for p.curr.Type == token.TokMap.Type("kwdVars") {
		if err := p.parseVarDeclaration(isFunction); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) parseVarDeclaration(isFunction bool) error {
	if err := p.expect(token.TokMap.Type("kwdVars")); err != nil {
		return err
	}

	if err := p.parseVarList(); err != nil {
		return err
	}

	// process varType in a later process

	if err := p.expect(token.TokMap.Type("terminator")); err != nil {
		return err
	}

	return p.parseVarDeclarationSection(isFunction)
}

func (p *Parser) parseVarList() error {
	currentVars := make([]string, 0)
	initialVar := string(p.curr.Lit)
	currentVars = append(currentVars, initialVar)
	if err := p.expect(token.TokMap.Type("id")); err != nil {
		return err
	}
	// Append id into currentVars

	for p.curr.Type == token.TokMap.Type("repeatTerminator") {
		p.next()
		currentVars = append(currentVars, string(p.curr.Lit))
		if err := p.expect(token.TokMap.Type("id")); err != nil {
			return err
		}
	}

	if err := p.expect(token.TokMap.Type("typeAssignOp")); err != nil {
		return err
	}

	currType := string(p.curr.Lit)
	semType, err := p.returnSemanticType(currType)
	if err != nil {
		return err
	}

	if err := p.addVariablesToSymbolTable(semType, currentVars); err != nil {
		return err
	}

	_, err = p.parseType()

	if err != nil {
		return err
	}

	return nil
}

func (p *Parser) parseType() (string, error) {
	currType := p.curr.Lit
	if err := p.expect(token.TokMap.Type("type")); err != nil {
		return "", err
	}
	return string(currType), nil
}

func (p *Parser) parseFunctionListOpt() error {
	if p.curr.Type != token.TokMap.Type("kwdFunc") {
		return nil
	}

	if p.curr.Type != token.TokMap.Type("kwdFunc") {
		if p.curr.Type != token.TokMap.Type("kwdBegin") {
			return fmt.Errorf("line %d: unexpected token '%s', expected 'var', 'func', or 'begin'",
				p.curr.Line, p.curr.Lit)
		}
		return nil
	}

	if err := p.parseFunctionList(); err != nil {
		return err
	}

	return nil
}

func (p *Parser) parseFunctionList() error {

	for p.curr.Type == token.TokMap.Type("kwdFunc") {
		if err := p.parseFunction(); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) parseFunction() error {

	if err := p.expect(token.TokMap.Type("kwdFunc")); err != nil {
		return err
	}

	functionId := p.curr.Lit

	if err := p.expect(token.TokMap.Type("id")); err != nil {
		return err
	}

	if err := p.expect(token.TokMap.Type("openParan")); err != nil {
		return err
	}

	p.codeGenerator.MemoryManager.PushNewFunctionSegment()

	params, err := p.parseParameterList()
	if err != nil {
		return err
	}

	if err := p.symbolTable.AddFunction(string(functionId), params, p.curr.Line, p.curr.Column); err != nil {
		return err
	}

	if err := p.symbolTable.EnterFunctionScope(string(functionId)); err != nil {
		return err
	}

	if err := p.expect(token.TokMap.Type("closeParan")); err != nil {
		return err
	}

	functionStartQuad := len(p.codeGenerator.Quads)
	if err := p.symbolTable.UpdateFunctionStartQuad(string(functionId), functionStartQuad); err != nil {
		return err
	}

	if err := p.parseFunctionBlock(); err != nil {
		return err
	}

	if err := p.codeGenerator.HandleENDPROC(); err != nil {
		return err
	}
	if err := p.codeGenerator.MemoryManager.PopFunctionSegment(); err != nil {
		return err
	}
	if err := p.expect(token.TokMap.Type("terminator")); err != nil {
		return err
	}

	p.symbolTable.ExitFunctionScope()
	return p.parseFunctionList()
}

func (p *Parser) parseParameterList() ([]semantic.Variable, error) {
	if p.curr.Type == token.TokMap.Type("closeParan") {
		return []semantic.Variable{}, nil
	}

	currentParams := make([]semantic.Variable, 0)
	currId := p.curr
	currType, err := p.parseParameter()
	if err != nil {
		return []semantic.Variable{}, err
	}

	semType, err := p.returnSemanticType(currType)
	addr, err := p.codeGenerator.MemoryManager.AllocateLocal(semType)

	if err != nil {
		return []semantic.Variable{}, err
	}

	currentParams = append(currentParams, semantic.Variable{
		Name:    string(currId.Lit),
		Type:    semType,
		Line:    currId.Line,
		Column:  currId.Column,
		Address: addr,
	})

	if err != nil {
		return []semantic.Variable{}, err
	}

	for p.curr.Type == token.TokMap.Type("repeatTerminator") {
		p.next() // consume the repeat terminator
		currId := p.curr
		currType, err := p.parseParameter()
		semType, err := p.returnSemanticType(currType)

		if err != nil {
			return []semantic.Variable{}, err
		}
		addr, err := p.codeGenerator.MemoryManager.AllocateLocal(semType)
		if err != nil {
			return []semantic.Variable{}, err
		}
		currentParams = append(currentParams, semantic.Variable{
			Name:    string(currId.Lit),
			Type:    semType,
			Line:    currId.Line,
			Column:  currId.Column,
			Address: addr,
		})

		if err != nil {
			return []semantic.Variable{}, err
		}
	}

	return currentParams, nil
}

func (p *Parser) parseParameter() (string, error) {
	if err := p.expect(token.TokMap.Type("id")); err != nil {
		return "", err
	}

	if err := p.expect(token.TokMap.Type("typeAssignOp")); err != nil {
		return "", err
	}

	currType, err := p.parseType()
	if err != nil {
		return "", err
	}

	return currType, nil
}

func (p *Parser) parseBlock() error {
	if err := p.expect(token.TokMap.Type("openBrace")); err != nil {
		return err
	}

	if err := p.parseStatementList(); err != nil {
		return err
	}

	return p.expect(token.TokMap.Type("closeBrace"))
}

func (p *Parser) parseFunctionBlock() error {
	if err := p.expect(token.TokMap.Type("openBrace")); err != nil {
		return err
	}

	if err := p.parseVarDeclarationSection(true); err != nil {
		return err
	}

	if err := p.parseStatementList(); err != nil {
		return err
	}

	return p.expect(token.TokMap.Type("closeBrace"))
}

// Parsing Statements
func (p *Parser) parseStatementList() error {
	for {
		validStatementStart, err := p.isStatementStart()
		if err != nil {
			return err
		}

		if !validStatementStart {
			break // No more statements to parse
		}

		if err := p.parseStatement(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) parseStatement() error {
	validStatementStart, err := p.isStatementStart()

	if !validStatementStart {
		return nil
	}

	if err != nil {
		return err
	}

	switch p.curr.Type {
	case token.TokMap.Type("kwdIf"):
		return p.parseIfStatement()
	case token.TokMap.Type("kwdWhile"):
		return p.parseWhileStatement()
	case token.TokMap.Type("kwdPrint"):
		return p.parsePrintStatement()
	case token.TokMap.Type("id"):
		// Functionality of function calls to be added

		idToken := p.curr
		p.next()
		nextToken := p.curr
		if nextToken.Type == token.TokMap.Type("openParan") {
			return p.parseFunctionCall(idToken)
		} else if nextToken.Type == token.TokMap.Type("assignOp") {
			if err := p.symbolTable.ValidateVarAssignment(string(idToken.Lit), idToken.Line); err != nil {
				return err
			}
			return p.parseAssignment(idToken, nextToken)
		} else {
			return fmt.Errorf("Expected either = or (, got %v at line %d, column %d", token.TokMap.Id(nextToken.Type), nextToken.Line, nextToken.Column)
		}
	}
	return nil
}

func (p *Parser) parseAssignment(id, nextToken *token.Token) error {
	if nextToken.Type != token.TokMap.Type("assignOp") {
		return fmt.Errorf("Expected =, got %v at line %d, column %d", nextToken.Type, nextToken.Line, nextToken.Column)
	}
	p.next()

	_, err := p.parseExpression()
	currType, err := p.symbolTable.GetType(string(id.Lit))
	if err != nil {
		return err
	}

	targetAddr, err := p.symbolTable.GetVariableAddress(string(id.Lit))
	if err != nil {
		return err
	}

	// fmt.Println("The expression type is ", exprType, "and the tok", string(id.Lit))
	if err := p.codeGenerator.HandleAssignment(targetAddr, currType); err != nil {
		return err
	}
	if err := p.expect(token.TokMap.Type("terminator")); err != nil {
		return err
	}

	return nil
}

func (p *Parser) parseWhileStatement() error {
	if err := p.expect(token.TokMap.Type("kwdWhile")); err != nil {
		return err
	}

	startIndex := p.codeGenerator.HandleWhileStart()

	if err := p.expect(token.TokMap.Type("openParan")); err != nil {
		return err
	}

	_, err := p.parseExpression()
	if err != nil {
		return err
	}

	// QUADS
	if err := p.codeGenerator.HandleWhileCondition(); err != nil {
		return err
	}

	if err := p.expect(token.TokMap.Type("closeParan")); err != nil {
		return err
	}

	if err := p.parseBlock(); err != nil {
		return err
	}

	if err := p.codeGenerator.HandleWhileEnd(startIndex); err != nil {
		return err
	}

	return nil
}

func (p *Parser) parseIfStatement() error {
	if err := p.expect(token.TokMap.Type("kwdIf")); err != nil {
		return err
	}

	if err := p.expect(token.TokMap.Type("openParan")); err != nil {
		return err
	}

	_, err := p.parseExpression()
	if err != nil {
		return err
	}

	if err := p.expect(token.TokMap.Type("closeParan")); err != nil {
		return err
	}

	// QUADS
	if err := p.codeGenerator.HandleIfStatement(); err != nil {
		return err
	}

	if err := p.parseBlock(); err != nil {
		return err
	}

	if p.curr.Type == token.TokMap.Type("kwdElse") {
		p.next()
		if err := p.codeGenerator.HandleElse(); err != nil {
			return err
		}

		if err := p.parseBlock(); err != nil {
			return err
		}

		if err := p.codeGenerator.HandleEndIf(); err != nil {
			return err
		}

		return nil
	}

	if err := p.codeGenerator.HandleEndIf(); err != nil {
		return err
	}

	return nil
}

func (p *Parser) parsePrintStatement() error {
	if err := p.expect(token.TokMap.Type("kwdPrint")); err != nil {
		return err
	}

	if err := p.expect(token.TokMap.Type("openParan")); err != nil {
		return err
	}

	if err := p.parsePrintList(); err != nil {
		return err
	}

	return p.expect(token.TokMap.Type("closeParan"))
}

func (p *Parser) parseFunctionCall(id *token.Token) error {
	functionName := string(id.Lit)

	if err := p.expect(token.TokMap.Type("openParan")); err != nil {
		return err
	}
	if err := p.codeGenerator.HandleERA(functionName); err != nil {
		return err
	}
	// Logic to verify with symbol table pending
	arguments, err := p.parseArgumentList()

	if err != nil {
		return err
	}

	if err := p.expect(token.TokMap.Type("closeParan")); err != nil {
		return err
	}

	if err := p.symbolTable.ValidateFunctionCall(string(id.Lit), id.Line, arguments); err != nil {
		return err
	}

	startQuad, err := p.symbolTable.GetFunctionStartQuad(string(id.Lit))

	if err != nil {
		return err
	}

	if err := p.codeGenerator.HandleGOSUB(string(id.Lit), startQuad); err != nil {
		return err
	}

	return nil
}

func (p *Parser) parseArgumentList() ([]shared.Type, error) {
	if p.curr.Type == token.TokMap.Type("closeParan") {
		return []shared.Type{}, nil
	}
	paramCount := 0
	argumentTypes := make([]shared.Type, 0)
	currType, err := p.parseExpression()
	if err != nil {
		return []shared.Type{}, err
	}

	if !p.codeGenerator.OperandStack.IsEmpty() {
		arg := p.codeGenerator.OperandStack.Pop()
		p.codeGenerator.TypeStack.Pop()
		if err := p.codeGenerator.HandleParam(arg, paramCount); err != nil {
			return []shared.Type{}, err
		}
		paramCount++
	}

	argumentTypes = append(argumentTypes, currType)

	for p.curr.Type == token.TokMap.Type("repeatTerminator") {
		p.next()
		argType, err := p.parseExpression()
		argumentTypes = append(argumentTypes, argType)
		if err != nil {
			return []shared.Type{}, err
		}

		if !p.codeGenerator.OperandStack.IsEmpty() {
			arg := p.codeGenerator.OperandStack.Pop()
			p.codeGenerator.TypeStack.Pop()
			if err := p.codeGenerator.HandleParam(arg, paramCount); err != nil {
				return []shared.Type{}, err
			}
			paramCount++
		}
		argumentTypes = append(argumentTypes, argType)
	}
	return argumentTypes, nil
}

func (p *Parser) parsePrintList() error {
	if err := p.parsePrintItem(); err != nil {
		return err
	}

	for p.curr.Type == token.TokMap.Type("repeatTerminator") {
		p.next()
		if err := p.parsePrintItem(); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) parsePrintItem() error {

	if p.curr.Type == token.TokMap.Type("stringLit") {
		if err := p.codeGenerator.HandlePrint(string(p.curr.Lit)); err != nil {
			return err
		}
		p.next()
		return nil
	}

	_, err := p.parseExpression()
	if err != nil {
		return err
	}

	if p.codeGenerator.OperandStack.IsEmpty() {
		return fmt.Errorf("missing expression result for print statement")
	}
	result := p.codeGenerator.OperandStack.Pop()
	p.codeGenerator.TypeStack.Pop()

	if err := p.codeGenerator.HandlePrint(result); err != nil {
		return err
	}

	if err != nil {
		return err
	}

	return nil
}

func (p *Parser) parseExpression() (shared.Type, error) {
	leftType, err := p.parseExp()
	if err != nil {
		return shared.TypeError, err
	}

	if p.curr.Type == token.TokMap.Type("relOp") {
		operator := string(p.curr.Lit)
		p.codeGenerator.OperatorStack.Push(operator)
		p.next()

		_, err := p.parseExp()
		if err != nil {
			return shared.TypeError, err
		}

		if err := p.codeGenerator.HandleOp(); err != nil {
			return shared.TypeError, err
		}

		return shared.TypeInt, nil
	}

	return leftType, nil
}

func (p *Parser) parseExp() (shared.Type, error) {
	leftType, err := p.parseTerm()
	if err != nil {
		return shared.TypeError, err
	}

	for p.curr.Type == token.TokMap.Type("expressionOp") {
		operator := string(p.curr.Lit)
		p.next()

		p.codeGenerator.OperatorStack.Push(operator)

		rightType, err := p.parseTerm()
		if err != nil {
			return shared.TypeError, err
		}

		if err := p.codeGenerator.HandleOp(); err != nil {
			return shared.TypeError, err
		}

		if leftType == shared.TypeFloat || rightType == shared.TypeFloat {
			leftType = shared.TypeFloat
		} else {
			leftType = shared.TypeInt
		}

	}

	return leftType, nil
}

func (p *Parser) parseTerm() (shared.Type, error) {
	leftType, err := p.parseFactor()
	if err != nil {
		return shared.TypeError, err
	}

	if p.curr.Type == token.TokMap.Type("termOp") {
		operator := string(p.curr.Lit)
		p.next()

		p.codeGenerator.OperatorStack.Push(operator)

		rightType, err := p.parseTerm()

		if err := p.codeGenerator.HandleOp(); err != nil {
			return shared.TypeError, err
		}

		if err != nil {
			return shared.TypeError, err
		}

		// Check compatibility using semantic cube
		if leftType == shared.TypeFloat || rightType == shared.TypeFloat {
			return shared.TypeFloat, nil
		}

		return shared.TypeInt, nil
	}

	return leftType, nil
}

func (p *Parser) parseFactor() (shared.Type, error) {
	switch p.curr.Type {
	case token.TokMap.Type("openParan"):
		p.codeGenerator.HandleOpenParen()
		p.next()
		exprType, err := p.parseExpression()
		if err != nil {
			return shared.TypeError, err
		}

		if p.curr.Type == token.TokMap.Type("closeParan") {
			if err := p.codeGenerator.HandleCloseParen(); err != nil {
				return exprType, err
			}
		}
		if err := p.expect(token.TokMap.Type("closeParan")); err != nil {
			return shared.TypeError, err
		}
		return exprType, nil
	case token.TokMap.Type("expressionOp"):
		litSymbol := string(p.curr.Lit)
		if string(p.curr.Lit) != "+" && string(p.curr.Lit) != "-" {
			return shared.TypeError, fmt.Errorf("unexpected operator: %s", p.curr.Lit)
		}
		p.next()

		switch p.curr.Type {
		// Logic for negatives??? Plus sign should be parsed but ignored.
		case token.TokMap.Type("id"), token.TokMap.Type("intLit"), token.TokMap.Type("floatLit"):
			tok := p.curr
			tokType, err := p.getType(p.curr)
			if err != nil {
				return shared.TypeError, err
			}
			if litSymbol == "+" {
				litSymbol = ""
			}

			if err := p.codeGenerator.HandleFactor(litSymbol+string(tok.Lit), tokType, p.symbolTable); err != nil {
				return shared.TypeError, err
			}
			return tokType, nil
		default:
			return shared.TypeError, fmt.Errorf("expected number after %s", p.curr.Lit)
		}
	case token.TokMap.Type("id"), token.TokMap.Type("intLit"), token.TokMap.Type("floatLit"):
		tok := p.curr
		tokType, err := p.getType(tok)
		if err != nil {
			return shared.TypeError, err
		}
		if err := p.codeGenerator.HandleFactor(string(tok.Lit), tokType, p.symbolTable); err != nil {
			return shared.TypeError, err
		}
		return tokType, nil
	default:
		return shared.TypeError, fmt.Errorf("unexpected token in factor: %v in line %l and %t",
			token.TokMap.Id(p.curr.Type), p.curr.Line, string(p.curr.Lit))
	}
}

func (p *Parser) parseMainSection() error {
	if err := p.expect(token.TokMap.Type("kwdBegin")); err != nil {
		return err
	}

	mainQuadIndex := len(p.codeGenerator.Quads)
	p.codeGenerator.Quads[0].Result = mainQuadIndex

	if err := p.parseStatementList(); err != nil {
		return err
	}

	if err := p.expect(token.TokMap.Type("kwdEnd")); err != nil {
		return err
	}

	return nil
}