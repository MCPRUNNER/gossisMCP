package main

import (
	"encoding/xml"
)

// SSISPackage represents the root of a DTSX file
type SSISPackage struct {
	XMLName               xml.Name              `xml:"Executable"`
	Properties            []Property            `xml:"Property"`
	ConnectionMgr         ConnectionMgr         `xml:"ConnectionManagers"`
	Variables             Variables             `xml:"Variables"`
	Executables           Executables           `xml:"Executables"`
	PrecedenceConstraints PrecedenceConstraints `xml:"PrecedenceConstraints"`
	EventHandlers         EventHandlers         `xml:"EventHandlers"`
	Parameters            Parameters            `xml:"Parameters"`
	Configurations        Configurations        `xml:"Configurations"`
}

type Property struct {
	Name  string `xml:"Name,attr"`
	Value string `xml:",innerxml"`
}

type ConnectionMgr struct {
	Connections []Connection `xml:"ConnectionManager"`
}

type Connection struct {
	Name       string     `xml:"ObjectName,attr"`
	ObjectData ObjectData `xml:"ObjectData"`
}

type ObjectData struct {
	ConnectionMgr InnerConnection `xml:"ConnectionManager"`
	MsmqConnMgr   MsmqConnection  `xml:"MsmqConnectionManager"`
}

type InnerConnection struct {
	ConnectionString string `xml:"ConnectionString,attr"`
}

type MsmqConnection struct {
	ConnectionString string `xml:"ConnectionString,attr"`
}

type Executables struct {
	Tasks []Task `xml:"Executable"`
}

// GetAllExecutables returns all tasks and containers as a unified slice
func (e *Executables) GetAllExecutables() []Task {
	return e.Tasks
}

type Executable struct {
	Name         string               `xml:"ObjectName,attr"`
	CreationName string               `xml:"CreationName,attr"`
	Description  string               `xml:"Description,attr"`
	RefId        string               `xml:"refId,attr"`
	Properties   []Property           `xml:"Property"`
	ObjectData   ExecutableObjectData `xml:"ObjectData"`
	Executables  *Executables         `xml:"Executables"` // For containers
	Variables    Variables            `xml:"Variables"`   // For containers
}

type ExecutableObjectData struct {
	// Task-specific data
	Task       TaskDetails       `xml:"Task"`
	ScriptTask ScriptTaskDetails `xml:"ScriptTask"`
	DataFlow   DataFlowDetails   `xml:"pipeline"`

	// Container-specific data
	SequenceContainer    SequenceContainerDetails    `xml:"SequenceContainer"`
	ForLoopContainer     ForLoopContainerDetails     `xml:"ForLoopContainer"`
	ForeachLoopContainer ForeachLoopContainerDetails `xml:"ForeachLoopContainer"`
}

type SequenceContainerDetails struct {
	// Sequence containers don't have specific properties beyond the base container
}

type ForLoopContainerDetails struct {
	ForLoop ForLoopDetails `xml:"ForLoop"`
}

type ForLoopDetails struct {
	InitExpression   string `xml:"InitExpression"`
	EvalExpression   string `xml:"EvalExpression"`
	AssignExpression string `xml:"AssignExpression"`
}

type ForeachLoopContainerDetails struct {
	ForeachLoop ForeachLoopDetails `xml:"ForeachLoop"`
}

type ForeachLoopDetails struct {
	Enumerator           string                      `xml:"Enumerator,attr"`
	CollectionEnumerator CollectionEnumeratorDetails `xml:"CollectionEnumerator"`
	ItemEnumerator       ItemEnumeratorDetails       `xml:"ItemEnumerator"`
	FileEnumerator       FileEnumeratorDetails       `xml:"FileEnumerator"`
	VariableMappings     VariableMappings            `xml:"VariableMappings"`
}

type CollectionEnumeratorDetails struct {
	Items []CollectionItem `xml:"Items>Item"`
}

type CollectionItem struct {
	Value string `xml:",innerxml"`
}

type ItemEnumeratorDetails struct {
	Items []CollectionItem `xml:"Items>Item"`
}

type FileEnumeratorDetails struct {
	Folder   string `xml:"Folder"`
	FileSpec string `xml:"FileSpec"`
	Recurse  bool   `xml:"Recurse"`
}

type VariableMappings struct {
	Mappings []VariableMapping `xml:"VariableMapping"`
}

type VariableMapping struct {
	VariableName string `xml:"VariableName,attr"`
	Index        int    `xml:"Index,attr"`
}

type Task struct {
	Name         string         `xml:"ObjectName,attr"`
	CreationName string         `xml:"CreationName,attr"`
	Description  string         `xml:"Description,attr"`
	RefId        string         `xml:"refId,attr"`
	Properties   []Property     `xml:"Property"`
	ObjectData   TaskObjectData `xml:"ObjectData"`
}

type TaskObjectData struct {
	Task       TaskDetails       `xml:"Task"`
	ScriptTask ScriptTaskDetails `xml:"ScriptTask"`
	DataFlow   DataFlowDetails   `xml:"pipeline"`
}

type TaskDetails struct {
	MessageQueueTask MessageQueueTaskDetails `xml:"MessageQueueTask"`
}

type MessageQueueTaskDetails struct {
	MessageQueueTaskData MessageQueueTaskData `xml:"MessageQueueTaskData"`
}

type MessageQueueTaskData struct {
	MessageType string `xml:"MessageType,attr"`
	Message     string `xml:"Message"`
}

type ScriptTaskDetails struct {
	ScriptTaskData ScriptTaskData `xml:"ScriptTaskData"`
}

type ScriptTaskData struct {
	ScriptProject ScriptProject `xml:"ScriptProject"`
}

type ScriptProject struct {
	ScriptCode string `xml:",innerxml"`
}

type DataFlowDetails struct {
	Components DataFlowComponents `xml:"components"`
	Paths      DataFlowPaths      `xml:"paths"`
}

type DataFlowComponents struct {
	Components []DataFlowComponent `xml:"component"`
}

type DataFlowComponent struct {
	Name                     string              `xml:"name,attr"`
	ComponentClassID         string              `xml:"componentClassID,attr"`
	Description              string              `xml:"description,attr"`
	LocaleID                 string              `xml:"localeId,attr"`
	UsesDispositions         bool                `xml:"usesDispositions,attr"`
	ValidateExternalMetadata bool                `xml:"validateExternalMetadata,attr"`
	Version                  int                 `xml:"version,attr"`
	ObjectData               ComponentObjectData `xml:"objectData"`
	Inputs                   ComponentInputs     `xml:"inputs"`
	Outputs                  ComponentOutputs    `xml:"outputs"`
}

type ComponentObjectData struct {
	PipelineComponent PipelineComponent `xml:"pipelineComponent"`
}

type PipelineComponent struct {
	Properties ComponentProperties `xml:"properties"`
}

type ComponentProperties struct {
	Properties []ComponentProperty `xml:"property"`
}

type ComponentProperty struct {
	Name  string `xml:"Name,attr"`
	Value string `xml:",innerxml"`
}

type ComponentInputs struct {
	Inputs []ComponentInput `xml:"input"`
}

type ComponentInput struct {
	Name           string       `xml:"name,attr"`
	HasSideEffects bool         `xml:"hasSideEffects,attr"`
	IsSorted       bool         `xml:"isSorted,attr"`
	InputColumns   InputColumns `xml:"inputColumns"`
}

type InputColumns struct {
	Columns []InputColumn `xml:"inputColumn"`
}

type InputColumn struct {
	Name      string `xml:"name,attr"`
	DataType  string `xml:"dataType,attr"`
	Length    int    `xml:"length,attr"`
	Precision int    `xml:"precision,attr"`
	Scale     int    `xml:"scale,attr"`
	CodePage  int    `xml:"codePage,attr"`
}

type ComponentOutputs struct {
	Outputs []ComponentOutput `xml:"output"`
}

type ComponentOutput struct {
	Name           string        `xml:"name,attr"`
	HasSideEffects bool          `xml:"hasSideEffects,attr"`
	IsErrorOut     bool          `xml:"isErrorOut,attr"`
	Synchronous    bool          `xml:"synchronous,attr"`
	OutputColumns  OutputColumns `xml:"outputColumns"`
}

type OutputColumns struct {
	Columns []OutputColumn `xml:"outputColumn"`
}

type OutputColumn struct {
	Name      string `xml:"name,attr"`
	DataType  string `xml:"dataType,attr"`
	Length    int    `xml:"length,attr"`
	Precision int    `xml:"precision,attr"`
	Scale     int    `xml:"scale,attr"`
	CodePage  int    `xml:"codePage,attr"`
}

type DataFlowPaths struct {
	Paths []DataFlowPath `xml:"Path"`
}

type DataFlowPath struct {
	Name    string `xml:"name,attr"`
	StartID string `xml:"startId,attr"`
	EndID   string `xml:"endId,attr"`
}

type Variables struct {
	Vars []Variable `xml:"Variable"`
}

type Variable struct {
	Name       string `xml:"ObjectName,attr"`
	Value      string `xml:"VariableValue"`
	Expression string `xml:"Expression,attr"`
}

type PrecedenceConstraints struct {
	Constraints []PrecedenceConstraint `xml:"PrecedenceConstraint"`
}

type PrecedenceConstraint struct {
	Name       string `xml:"ObjectName,attr"`
	From       string `xml:"From,attr"`
	To         string `xml:"To,attr"`
	Expression string `xml:"Expression,attr"`
	EvalOp     string `xml:"EvalOp,attr"`
}

type EventHandlers struct {
	EventHandlers []EventHandler `xml:"EventHandler"`
}

type EventHandler struct {
	EventHandlerType      string                `xml:"EventHandlerType,attr"`
	ContainerID           string                `xml:"ContainerID,attr"`
	ObjectName            string                `xml:"ObjectName,attr"`
	Executables           Executables           `xml:"Executables"`
	Variables             Variables             `xml:"Variables"`
	PrecedenceConstraints PrecedenceConstraints `xml:"PrecedenceConstraints"`
}

type Parameters struct {
	Params []Parameter `xml:"Parameter"`
}

type Parameter struct {
	Name        string `xml:"ObjectName,attr"`
	DataType    string `xml:"DataType,attr"`
	Value       string `xml:"ParameterValue"`
	Description string `xml:"Description,attr"`
	Required    bool   `xml:"Required,attr"`
	Sensitive   bool   `xml:"Sensitive,attr"`
}

type Configurations struct {
	Configs []Configuration `xml:"Configuration"`
}

type Configuration struct {
	Name                string `xml:"ObjectName,attr"`
	Type                int    `xml:"ConfigurationType,attr"`
	Description         string `xml:"Description,attr"`
	ConfigurationString string `xml:"ConfigurationString"`
	ConfiguredType      string `xml:"ConfiguredType,attr"`
	ConfiguredValue     string `xml:"ConfiguredValue"`
}

type PerformanceMetrics struct {
	PackageLevel   []PerformanceProperty
	DataFlowLevel  []DataFlowPerformance
	ComponentLevel []ComponentPerformance
}

type PerformanceProperty struct {
	Name           string
	Value          string
	Category       string
	Recommendation string
}

type DataFlowPerformance struct {
	TaskName   string
	Properties []PerformanceProperty
	Components []ComponentPerformance
}

type ComponentPerformance struct {
	ComponentName string
	ComponentType string
	Properties    []PerformanceProperty
}
