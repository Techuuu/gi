// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Originally written by Kai O'Reilly (https://github.com/kkoreilly)

package main

// TODO: fix
/*
// todo: investigate "github.com/antonmedv/expr" instead of govaluate -- much better supported!

var colornames = []string{"black", "red", "blue", "green", "purple", "brown", "orange"}

// Graph represents the overall graph parameters -- lines and params
type Graph struct {

	// the parameters for updating the marbles
	Params Params `desc:"the parameters for updating the marbles"`

	// [view: -] the lines of the graph -- can have any number
	Lines Lines `view:"-" desc:"the lines of the graph -- can have any number"`
}

// Gr is current graph
var Gr Graph

// GraphProps define the Toolbar for overall app
var GraphProps = ki.Props{
	"Toolbar": ki.PropSlice{
		{"OpenJSON", ki.Props{
			"label": "Open...",
			"desc":  "Opens line equations and params from a .json file.",
			"icon":  icons.Open,
			"Args": ki.PropSlice{
				{"File Name", ki.Props{
					"ext": ".json",
				}},
			},
		}},
		{"SaveJSON", ki.Props{
			"label": "Save As...",
			"desc":  "Saves line equations and params to a .json file.",
			"icon":  icons.SaveAs,
			"Args": ki.PropSlice{
				{"File Name", ki.Props{
					"ext": ".json",
				}},
			},
		}},
		{"sep-ctrl", ki.BlankProp{}},
		{"Graph", ki.Props{
			"desc": "updates graph for current equations",
			"icon": icons.Image,
		}},
		{"Run", ki.Props{
			"desc":            "runs the marbles for NSteps",
			"icon":            icons.PlayCircle,
			"no-update-after": true,
		}},
		{"Stop", ki.Props{
			"desc":            "runs the marbles for NSteps",
			"icon":            icons.Stop,
			"no-update-after": true,
		}},
		{"Step", ki.Props{
			"desc":            "steps the marbles for one step",
			"icon":            icons.KeyboardArrowRight,
			"no-update-after": true,
		}},
		{"Reset", ki.Props{
			"desc": "resets marbles to their initial starting positions",
			"icon": icons.DeviceReset,
		}},
	},
}

func (gr *Graph) Defaults() {
	gr.Params.Defaults()
	gr.Lines.Defaults()
}

// OpenJSON open from JSON file
func (gr *Graph) OpenJSON(filename gi.FileName) error {
	b, err := os.ReadFile(string(filename))
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, gr)
	gr.Reset()
	return err
}

// SaveJSON save to JSON file
func (gr *Graph) SaveJSON(filename gi.FileName) error {
	b, err := json.MarshalIndent(gr, "", "  ")
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	err = os.WriteFile(string(filename), b, 0644)
	if err != nil {
		slog.Error(err.Error())
	}
	return err
}

// Graph updates graph for current equations, and resets marbles too
func (gr *Graph) Graph() {
	// ResetMarbles()
	gr.Lines.Graph()
}

// Run runs the marbles for NSteps
func (gr *Graph) Run() {
	go RunMarbles()
}

// Stop stops the marbles
func (gr *Graph) Stop() {
	Stop = true
}

// Step does one step update of marbles
func (gr *Graph) Step() {
	UpdateMarbles()
}

// Reset resets the marbles to their initial starting positions
func (gr *Graph) Reset() {
	ResetMarbles()
	gr.Lines.Graph()
}

///////////////////////////////////////////////////////////////////////////
//  Lines

// Line represents one line with an equation etc
type Line struct {

	// equation: use 'x' for the x value, and must use * for multiplication, and start with 0 for decimal numbers (0.01 instead of .01)
	Eq string `width:"60" desc:"equation: use 'x' for the x value, and must use * for multiplication, and start with 0 for decimal numbers (0.01 instead of .01)"`

	// [step: 1] Minimum x value for this line.
	MinX float32 `step:"1" desc:"Minimum x value for this line."`

	// [step: 1] Maximum x value for this line.
	MaxX float32 `step:"1" desc:"Maximum x value for this line."`

	// color to draw the line in
	Color string `desc:"color to draw the line in"`

	// [min: 0] [max: 2] [step: .05] how bouncy the line is -- 1 = perfectly bouncy, 0 = no bounce at all
	Bounce float32 `min:"0" max:"2" step:".05" desc:"how bouncy the line is -- 1 = perfectly bouncy, 0 = no bounce at all"`

	// [tableview: -] the expression evaluator
	expr *govaluate.EvaluableExpression `tableview:"-" desc:"the expression evaluator"`

	// [tableview: -] the eval params
	params map[string]any `tableview:"-" desc:"the eval params"`
}

func (ln *Line) Defaults(lidx int) {
	ln.Eq = "x"
	ln.Color = colornames[lidx%len(colornames)]
	ln.Bounce = 0.95
	ln.MinX = -10
	ln.MaxX = 10
}

// Eval gives the y value of the function for given x value
func (ln *Line) Eval(x float32) float32 {
	if ln.expr == nil {
		return 0
	}
	if ln.params == nil {
		ln.params = make(map[string]any, 1)
	}
	ln.params["x"] = float64(x)
	yi, _ := ln.expr.Evaluate(ln.params)
	y := float32(yi.(float64))
	return y
}

// Lines is a collection of lines
type Lines []*Line

// LinesProps define the Toolbar for lines
var LinesProps = ki.Props{
	// "Toolbar": ki.PropSlice{
	// 	{"OpenJSON", ki.Props{
	// 		"label": "Open...",
	// 		"desc":  "opens equations from a .json file.",
	// 		"icon":  "file-open",
	// 		"Args": ki.PropSlice{
	// 			{"File Name", ki.Props{
	// 				"ext": ".json",
	// 			}},
	// 		},
	// 	}},
	// 	{"SaveJSON", ki.Props{
	// 		"label": "Save As...",
	// 		"desc":  "Saves equations from a .json file.",
	// 		"icon":  "file-save",
	// 		"Args": ki.PropSlice{
	// 			{"File Name", ki.Props{
	// 				"ext": ".json",
	// 			}},
	// 		},
	// 	}},
	// },
}

func (ls *Lines) Defaults() {
	*ls = make(Lines, 1, 10)
	ln := Line{}
	(*ls)[0] = &ln
	ln.Defaults(0)

}

// OpenJSON open from JSON file
func (ls *Lines) OpenJSON(filename gi.FileName) error {
	b, err := os.ReadFile(string(filename))
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, ls)
	return err
}

// SaveJSON save to JSON file
func (ls *Lines) SaveJSON(filename gi.FileName) error {
	b, err := json.MarshalIndent(ls, "", "  ")
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	err = os.WriteFile(string(filename), b, 0644)
	if err != nil {
		slog.Error(err.Error())
	}
	return err
}

func (ls *Lines) Graph() {
	updt := SvgGraph.UpdateStart()
	SvgLines.DeleteChildren(true)
	for i, ln := range *ls {
		ln.Graph(i)
	}
	SvgGraph.UpdateEnd(updt)
}

// Graph graphs this line in the SvgLines group
func (ln *Line) Graph(lidx int) {
	if ln.Eq == "" {
		ln.Defaults(lidx)
	}
	if ln.Color == "" {
		ln.Color = colornames[lidx%len(colornames)]
	}
	if ln.Bounce == 0 {
		ln.Bounce = 0.95
	}
	if ln.MinX == 0 {
		ln.MinX = -10
	}
	if ln.MaxX == 0 {
		ln.MaxX = 10
	}
	path := svg.NewPath(SvgLines, "path", "")
	path.SetProp("fill", "none")
	clr := ln.Color
	path.SetProp("stroke", clr)

	var err error
	ln.expr, err = govaluate.NewEvaluableExpressionWithFunctions(ln.Eq, functions)
	if err != nil {
		ln.expr = nil
		slog.Error(err.Error())
		return
	}

	ps := ""
	start := true
	for x := gmin.X; x < gmax.X; x += ginc.X {

		if x > ln.MinX && x < ln.MaxX {
			y := ln.Eval(x)
			if start {
				ps += fmt.Sprintf("M %v %v ", x, y)
				start = false
			} else {
				ps += fmt.Sprintf("L %v %v ", x, y)
			}
		}
	}
	path.SetData(ps)
}

func InitCoords() {
	updt := SvgGraph.UpdateStart()
	SvgCoords.DeleteChildren(true)

	xAxis := svg.NewLine(SvgCoords, "xAxis", -10, 0, 10, 0)
	xAxis.SetProp("stroke", "#888")

	yAxis := svg.NewLine(SvgCoords, "yAxis", 0, -10, 0, 10)
	yAxis.SetProp("stroke", "#888")

	SvgGraph.UpdateEnd(updt)
}

/////////////////////////////////////////////////////////////////////////

//  Marbles

type Marble struct {
	Pos    mat32.Vec2
	Vel    mat32.Vec2
	PrvPos mat32.Vec2
}

func (mb *Marble) Init(diff float32) {
	randNum := (rand.Float32() * 2) - 1
	xPos := randNum * Gr.Params.Width
	mb.Pos = mat32.Vec2{xPos, 10 - diff}
	mb.Vel = mat32.Vec2{0, float32(-Gr.Params.StartSpeed)}
	mb.PrvPos = mb.Pos
}

var Marbles []*Marble

// Kai: put all these in a struct, and add a StructInlineView to edit them.
// see your other code for how to do it..

// Params holds our parameters
type Params struct {

	// [min: 1] [max: 10000] [step: 10] number of marbles
	NMarbles int `min:"1" max:"10000" step:"10" desc:"number of marbles"`

	// [min: 100] [max: 10000] [step: 10] number of steps to take when running
	NSteps int `min:"100" max:"10000" step:"10" desc:"number of steps to take when running"`

	// [min: 0] [max: 2] [step: .05] Coordinates per unit of time
	StartSpeed float32 `min:"0" max:"2" step:".05" desc:"Coordinates per unit of time"`

	// [min: 0.001] [max: 1] [step: .01] how fast to move along velocity vector -- lower = smoother, more slow-mo
	UpdtRate float32 `min:"0.001" max:"1" step:".01" desc:"how fast to move along velocity vector -- lower = smoother, more slow-mo"`

	// [min: 0] [max: 2] [step: .01] how fast it accelerates down
	Gravity float32 `min:"0" max:"2" step:".01" desc:"how fast it accelerates down"`
	Width   float32 `length of spawning zone for marbles, set to 0 for all spawn in a column`
}

func (pr *Params) Defaults() {
	pr.NMarbles = 10
	pr.NSteps = 10000
	pr.StartSpeed = 0
	pr.UpdtRate = .02
	pr.Gravity = 0.1
}

var MarbleRadius = .1

func RadToDeg(rad float32) float32 {
	return rad * 180 / math.Pi
}

// GraphMarblesInit initializes the graph drawing of the marbles
func GraphMarblesInit() {
	updt := SvgGraph.UpdateStart()

	SvgMarbles.DeleteChildren(true)
	for i, m := range Marbles {
		circle := svg.NewCircle(SvgMarbles, "circle", m.Pos.X, m.Pos.Y, float32(MarbleRadius))
		circle.SetProp("stroke", "none")
		circle.SetProp("fill", colornames[i%len(colornames)])
	}
	SvgGraph.UpdateEnd(updt)
}

// InitMarbles creates the marbles and puts them at their initial positions
func InitMarbles() {
	Marbles = make([]*Marble, 0)
	for n := 0; n < Gr.Params.NMarbles; n++ {
		diff := 2 * float32(n) / float32(Gr.Params.NMarbles)
		m := Marble{}
		m.Init(diff)
		Marbles = append(Marbles, &m)
	}
}

// ResetMarbles just calls InitMarbles and GraphMarblesInit
func ResetMarbles() {
	InitMarbles()
	GraphMarblesInit()
}

func UpdateMarbles() {
	wupdt := SvgGraph.TopUpdateStart()
	defer SvgGraph.TopUpdateEnd(wupdt)

	updt := SvgGraph.UpdateStart()
	defer SvgGraph.UpdateEnd(updt)
	SvgGraph.SetNeedsFullRender()

	for i, m := range Marbles {

		m.Vel.Y -= Gr.Params.Gravity
		npos := m.Pos.Add(m.Vel.MulScalar(Gr.Params.UpdtRate))
		ppos := m.Pos

		for _, ln := range Gr.Lines {
			if ln.expr == nil {
				continue
			}

			yp := ln.Eval(m.Pos.X)
			yn := ln.Eval(npos.X)

			// fmt.Printf("y: %v npos: %v pos: %v\n", y, npos.Y, m.Pos.Y)

			if ((npos.Y < yn && m.Pos.Y >= yp) || (npos.Y > yn && m.Pos.Y <= yp)) && (npos.X < ln.MaxX && npos.X > ln.MinX) {
				// fmt.Printf("Collided! Equation is: %v \n", ln.Eq)

				dly := yn - yp // change in the lines y
				dx := npos.X - m.Pos.X

				var yi, xi float32

				if dx == 0 {

					xi = npos.X
					yi = yn

				} else {

					ml := dly / dx
					dmy := npos.Y - m.Pos.Y
					mm := dmy / dx

					xi = (npos.X*(ml-mm) + npos.Y - yn) / (ml - mm)
					yi = ln.Eval(xi)
					//		fmt.Printf("xi: %v, yi: %v \n", xi, yi)
				}

				yl := ln.Eval(xi - .01) // point to the left of x
				yr := ln.Eval(xi + .01) // point to the right of x

				//slp := (yr - yl) / .02
				angLn := mat32.Atan2(yr-yl, 0.02)
				angN := angLn + math.Pi/2 // + 90 deg

				angI := mat32.Atan2(m.Vel.Y, m.Vel.X)
				angII := angI + math.Pi

				angNII := angN - angII
				angR := math.Pi + 2*angNII

				// fmt.Printf("angLn: %v  angN: %v  angI: %v  angII: %v  angNII: %v  angR: %v\n",
				// 	RadToDeg(angLn), RadToDeg(angN), RadToDeg(angI), RadToDeg(angII), RadToDeg(angNII), RadToDeg(angR))

				nvx := ln.Bounce * (m.Vel.X*mat32.Cos(angR) - m.Vel.Y*mat32.Sin(angR))
				nvy := ln.Bounce * (m.Vel.X*mat32.Sin(angR) + m.Vel.Y*mat32.Cos(angR))

				m.Vel = mat32.Vec2{nvx, nvy}

				m.Pos = mat32.Vec2{xi, yi}

			}
		}

		m.PrvPos = ppos
		m.Pos = m.Pos.Add(m.Vel.MulScalar(Gr.Params.UpdtRate))

		circle := SvgMarbles.Child(i).(*svg.Circle)
		circle.Pos = m.Pos
	}
}

var Stop = false

func RunMarbles() {
	Stop = false
	for i := 0; i < Gr.Params.NSteps; i++ {
		UpdateMarbles()
		if Stop {
			break
		}
	}
}

var functions = map[string]govaluate.ExpressionFunction{
	"cos": func(args ...any) (any, error) {
		y := math.Cos(args[0].(float64))
		return y, nil
	},
	"sin": func(args ...any) (any, error) {
		y := math.Sin(args[0].(float64))
		return y, nil
	},
	"tan": func(args ...any) (any, error) {
		y := math.Tan(args[0].(float64))
		return y, nil
	},
	"pow": func(args ...any) (any, error) {
		y := math.Pow(args[0].(float64), args[1].(float64))
		return y, nil
	},
	"abs": func(args ...any) (any, error) {
		y := math.Abs(args[0].(float64))
		return y, nil
	},
	"fact": func(args ...any) (any, error) {
		y := FactorialMemoization(int(args[0].(float64)))
		return y, nil
	},
}

const LIM = 100

var facts [LIM]float64

func FactorialMemoization(n int) (res float64) {
	if n < 0 {
		return 1
	}
	if facts[n] != 0 {
		res = facts[n]
		return res
	}
	if n > 0 {
		res = float64(n) * FactorialMemoization(n-1)
		return res
	}
	return 1
}
*/
