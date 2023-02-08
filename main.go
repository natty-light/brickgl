package main

import (
	"log"
	"runtime"
	"time"
	"unsafe"

	"github.com/EngoEngine/math"
	"github.com/engoengine/glm"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.1/glfw"
)

var (
	// global rotation
	width, height      int = 800, 800
	vertexShaderSource     = `
#version 410 core
layout (location = 0) in vec3 position;
layout (location = 1) in vec3 normal;

uniform mat4 transform;

out vec3 Normal;
out vec3 FragPos;

void main()
{
    gl_Position = transform*vec4(position.x, position.y, position.z, 1.0);
	Normal = normal;
	FragPos = vec3(transform*vec4(position.x, position.y, position.z, 1.0));
}
`
	fragmentShaderSource = `
#version 410 core
in vec3 Normal;
in vec3 FragPos;

uniform vec3 objectColor;
uniform vec3 lightColor;
uniform vec3 lightPos;
uniform float ambientStrength;

out vec4 color;

void main()
{
	float ambientStrength = 0.2;
	vec3 ambient = ambientStrength * lightColor;
	vec3 norm = normalize(Normal);
	vec3 lightDir = normalize(lightPos - FragPos); 
	float diff = max(dot(norm, lightDir), 0.0);
	vec3 diffuse = diff * lightColor; 
	vec3 result = (ambient + diffuse) * objectColor;
	color = vec4(result, 1.0);
}
`
	// Vertex definitions
	L1 = []float32{-0.25, 0.125, -0.5}
	L2 = []float32{-0.25, -0.125, -0.5}
	R1 = []float32{0.25, 0.125, -0.5}
	R2 = []float32{0.25, -0.125, -0.5}

	L3 = []float32{-0.25, 0.125, 0.5}
	L4 = []float32{-0.25, -0.125, 0.5}
	R3 = []float32{0.25, 0.125, 0.5}
	R4 = []float32{0.25, -0.125, 0.5}

	// Vertex Normal Vector defintions
	posXNorm = []float32{1.0, 0.0, 0.0}
	negXNorm = []float32{-1.0, 0.0, 0.0}
	posYNorm = []float32{0.0, 1.0, 0.0}
	negYNorm = []float32{0.0, -1.0, 0.0}
	posZNorm = []float32{0.0, 0.0, 1.0}
	negZNorm = []float32{0.0, 0.0, -1.0}
)

type getGlParam func(uint32, uint32, *int32)
type getInfoLog func(uint32, int32, *int32, *uint8)

func checkGlError(glObject uint32, errorParam uint32, getParamFn getGlParam,
	getInfoLogFn getInfoLog, failMsg string) {

	var success int32
	getParamFn(glObject, errorParam, &success)
	if success != 1 {
		var infoLog [512]byte
		getInfoLogFn(glObject, 512, nil, (*uint8)(unsafe.Pointer(&infoLog)))
		log.Fatalln(failMsg, "\n", string(infoLog[:512]))
	}
}

func checkShaderCompileErrors(shader uint32) {
	checkGlError(shader, gl.COMPILE_STATUS, gl.GetShaderiv, gl.GetShaderInfoLog,
		"ERROR::SHADER::COMPILE_FAILURE")
}

func checkProgramLinkErrors(program uint32) {
	checkGlError(program, gl.LINK_STATUS, gl.GetProgramiv, gl.GetProgramInfoLog,
		"ERROR::PROGRAM::LINKING_FAILURE")
}

func compileShaders(vertShaderSource string, fragShaderSource string) []uint32 {
	// create the vertex shader
	vertexShader := gl.CreateShader(gl.VERTEX_SHADER)
	shaderSourceChars, freeVertexShaderFunc := gl.Strs(vertShaderSource)
	gl.ShaderSource(vertexShader, 1, shaderSourceChars, nil)
	gl.CompileShader(vertexShader)
	checkShaderCompileErrors(vertexShader)

	// create the fragment shader
	fragmentShader := gl.CreateShader(gl.FRAGMENT_SHADER)
	shaderSourceChars, freeFragmentShaderFunc := gl.Strs(fragShaderSource)
	gl.ShaderSource(fragmentShader, 1, shaderSourceChars, nil)
	gl.CompileShader(fragmentShader)
	checkShaderCompileErrors(fragmentShader)

	defer freeFragmentShaderFunc()
	defer freeVertexShaderFunc()

	return []uint32{vertexShader, fragmentShader}
}

/*
 * Link the provided shaders in the order they were given and return the linked program.
 */
func linkShaders(shaders []uint32) uint32 {
	program := gl.CreateProgram()
	for _, shader := range shaders {
		gl.AttachShader(program, shader)
	}
	gl.LinkProgram(program)
	checkProgramLinkErrors(program)

	// shader objects are not needed after they are linked into a program object
	for _, shader := range shaders {
		gl.DeleteShader(shader)
	}

	return program
}

func createVAO(vertices []float32) (VAO uint32, VBO uint32) {

	gl.GenVertexArrays(1, &VAO)
	gl.GenBuffers(1, &VBO)

	// Bind the Vertex Array Object first, then bind and set vertex buffer(s) and attribute pointers()
	gl.BindVertexArray(VAO)

	// copy vertices data into VBO (it needs to be bound first)
	gl.BindBuffer(gl.ARRAY_BUFFER, VBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	// specify the format of our vertex input
	// (shader) input 0
	// vertex has size 3
	// vertex items are of type FLOAT
	// do not normalize (already done)
	// stride of 3 * sizeof(float) (separation of vertices)
	// offset of where the position data starts (0 for the beginning)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, 6*4, 0)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, 6*4, 3*4)
	gl.EnableVertexAttribArray(1)

	// unbind the VAO (safe practice so we don't accidentally (mis)configure it later)
	gl.BindVertexArray(0)

	return VAO, VBO
}

func reshape(window *glfw.Window, w, h int) {
	gl.ClearColor(1, 1, 1, 1)
	/* Establish viewing area to cover entire window. */
	gl.Viewport(0, 0, int32(w), int32(h))
	/* PROJECTION Matrix mode. */
	gl.MatrixMode(gl.PROJECTION)
	/* Reset project matrix. */
	gl.LoadIdentity()
	/* Map abstract coords directly to window coords. */
	gl.Ortho(0, float64(w), 0, float64(h), -1, 1)
	/* Invert Y axis so increasing Y goes down. */
	gl.Scalef(1, -1, 1)
	/* Shift origin up to upper-left corner. */
	gl.Translatef(0, float32(-h), 0)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.DEPTH_TEST)
	width, height = w, h
}

func init() {
	runtime.LockOSThread()
}

func main() {
	err := glfw.Init()
	if err != nil {
		panic(err)
	}
	defer glfw.Terminate()
	window, err := glfw.CreateWindow(width, height, "Brick", nil, nil)
	if err != nil {
		panic(err)
	}

	window.MakeContextCurrent()
	window.SetSizeCallback(reshape)
	window.SetKeyCallback(onKey)
	window.SetCharCallback(onChar)

	glfw.SwapInterval(-1)
	err = gl.Init()
	if err != nil {
		panic(err)
	}

	reshape(window, width, height)

	trongles := [][]float32{
		// Front Face
		constructTrongle([][]float32{L1, L2, R2}, posZNorm),
		constructTrongle([][]float32{R2, R1, L1}, posZNorm),
		// Back Face
		constructTrongle([][]float32{L3, L4, R4}, negZNorm),
		constructTrongle([][]float32{L3, R3, R4}, negZNorm),
		// Right Face
		constructTrongle([][]float32{R1, R2, R4}, negXNorm),
		constructTrongle([][]float32{R1, R3, R4}, negXNorm),
		// Left Face
		constructTrongle([][]float32{L1, L2, L4}, negXNorm),
		constructTrongle([][]float32{L1, L3, L4}, negXNorm),
		// Top Face
		constructTrongle([][]float32{L1, R1, R3}, negYNorm),
		constructTrongle([][]float32{L1, L3, R3}, negYNorm),
		// Bottom Face
		constructTrongle([][]float32{L2, R2, R4}, posYNorm),
		constructTrongle([][]float32{L2, L4, R4}, posYNorm),
	}
	var vertices []float32 = []float32{}
	for _, trongle := range trongles {
		vertices = append(vertices, trongle...)
	}

	shaders := compileShaders(vertexShaderSource, fragmentShaderSource)
	shaderProgram := linkShaders(shaders)

	var VAO, VBO uint32 = createVAO(vertices)

	// Lighting
	lightPos := glm.Vec3{1.2, 1.0, -2.0}
	lightColor := glm.Vec3{1.0, 1.0, 1.0}
	objectColor := glm.Vec3{1.0, 0.0, 0.0}

	// Animation

	transformation := glm.NewTransform()
	transformation.Iden()

	// First rotation about X axis
	axis := glm.Vec3{1.0, 0.0, 0.0}
	angle := float32(math.Pi / 8.0)

	rotationQuat := &glm.Quat{W: angle, V: axis}
	rotationQuat.Normalize()
	transformation.RotateQuat(rotationQuat)
	// Set up animiation rotation
	axis = glm.Vec3{0.0, 1, 0.0}
	angle = float32(math.Pi / 360.0)

	rotationQuat = &glm.Quat{W: angle, V: axis}
	rotationQuat.Normalize()
	gl.PolygonMode(gl.FRONT_FACE, gl.FILL)

	var lightColorLocation, objectColorLocation, lightPosLocation, transformLocation int32
	for !window.ShouldClose() {
		// Animation
		//transformation = glm.HomogRotate3D(t*math.Pi/180., &glm.Vec3{0.0, 1.0, 0})
		transformation.RotateQuat(rotationQuat)
		transformLocation = gl.GetUniformLocation(shaderProgram, gl.Str("transform\x00"))
		gl.UniformMatrix4fv(transformLocation, 1, false, &transformation[0])

		lightPosLocation = gl.GetUniformLocation(shaderProgram, gl.Str("lightPos\x00"))
		objectColorLocation = gl.GetUniformLocation(shaderProgram, gl.Str("objectColor\x00"))
		lightColorLocation = gl.GetUniformLocation(shaderProgram, gl.Str("lightColor\x00"))

		gl.Uniform3fv(lightPosLocation, 1, &lightPos[0])
		gl.Uniform3fv(lightColorLocation, 1, &lightColor[0])
		gl.Uniform3fv(objectColorLocation, 1, &objectColor[0])

		// perform rendering
		gl.ClearColor(0.0, 0.0, 0.0, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		gl.UseProgram(shaderProgram)                           // ensure the right shader program is being used
		gl.BindVertexArray(VAO)                                // bind data
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(vertices)/6)) // perform draw call
		gl.BindVertexArray(0)                                  // unbind data (so we don't mistakenly use/modify it)
		// end of draw loop

		// swap in the rendered buffer
		window.SwapBuffers()
		glfw.PollEvents()
		time.Sleep(16 * time.Millisecond)
	}
	gl.DeleteVertexArrays(1, &VAO)
	gl.DeleteBuffers(1, &VBO)
}

func onChar(w *glfw.Window, char rune) {
	log.Println(char)
}

// Keyboard key callback
func onKey(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	switch {
	case key == glfw.KeyEscape && action == glfw.Press,
		key == glfw.KeyQ && action == glfw.Press:
		w.SetShouldClose(true)
	}
}

func constructTrongle(points [][]float32, normal []float32) (trongle []float32) {
	for _, point := range points {
		trongle = append(trongle, point...)
		trongle = append(trongle, normal...)
	}
	return trongle
}
