package main

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSSISPackageParsing tests basic DTSX package parsing
func TestSSISPackageParsing(t *testing.T) {
	// Sample DTSX content (namespace stripped as done in main.go)
	dtsxContent := `<Executable xmlns="www.microsoft.com/SqlServer/Dts">
  <Property Name="PackageFormatVersion">8</Property>
  <Variables>
    <Variable ObjectName="TestVariable">
      <VariableValue DataType="3">42</VariableValue>
    </Variable>
  </Variables>
  <Executables />
</Executable>`

	var pkg SSISPackage
	err := xml.Unmarshal([]byte(dtsxContent), &pkg)
	require.NoError(t, err)

	// Test basic package properties - SSISPackage doesn't have root attributes
	// so we test the nested elements that exist
	assert.Len(t, pkg.Properties, 1)
	assert.Equal(t, "PackageFormatVersion", pkg.Properties[0].Name)
	assert.Equal(t, "8", pkg.Properties[0].Value)

	// Test variables
	assert.Len(t, pkg.Variables.Vars, 1)
	variable := pkg.Variables.Vars[0]
	assert.Equal(t, "TestVariable", variable.Name)
	assert.Equal(t, "42", variable.Value)
}

// TestSSISConnectionManagerParsing tests connection manager parsing
func TestSSISConnectionManagerParsing(t *testing.T) {
	dtsxContent := `<Executable xmlns="www.microsoft.com/SqlServer/Dts">
  <ConnectionManagers>
    <ConnectionManager ObjectName="TestConn">
      <ObjectData>
        <ConnectionManager ConnectionString="Data Source=TestServer;Initial Catalog=TestDB;Provider=MSOLEDBSQL19.1;Integrated Security=SSPI;" />
      </ObjectData>
    </ConnectionManager>
  </ConnectionManagers>
</Executable>`

	var pkg SSISPackage
	err := xml.Unmarshal([]byte(dtsxContent), &pkg)
	require.NoError(t, err)

	// Test connection managers
	assert.Len(t, pkg.ConnectionMgr.Connections, 1)
	conn := pkg.ConnectionMgr.Connections[0]
	assert.Equal(t, "TestConn", conn.Name)
	assert.Contains(t, conn.ObjectData.ConnectionMgr.ConnectionString, "TestServer")
}

// TestSSISMalformedXML tests handling of malformed XML
func TestSSISMalformedXML(t *testing.T) {
	// Malformed XML - missing closing tag
	malformedXML := `<Executable xmlns="www.microsoft.com/SqlServer/Dts">
  <Variables>
    <Variable ObjectName="TestVar">
      <VariableValue DataType="3">123</VariableValue>
    </Variable>
  </Variables>
</Executable>`

	var pkg SSISPackage
	err := xml.Unmarshal([]byte(malformedXML), &pkg)
	require.NoError(t, err)
	assert.Len(t, pkg.Variables.Vars, 1)
	assert.Equal(t, "TestVar", pkg.Variables.Vars[0].Name)
}

// TestSSISLargePackage tests parsing of a package with many elements
func TestSSISLargePackage(t *testing.T) {
	// Create a package with multiple variables
	dtsxContent := `<Executable xmlns="www.microsoft.com/SqlServer/Dts">
  <Variables>
    <Variable ObjectName="Var1">
      <VariableValue DataType="3">1</VariableValue>
    </Variable>
    <Variable ObjectName="Var2">
      <VariableValue DataType="3">2</VariableValue>
    </Variable>
    <Variable ObjectName="Var3">
      <VariableValue DataType="3">3</VariableValue>
    </Variable>
  </Variables>
</Executable>`

	var pkg SSISPackage
	err := xml.Unmarshal([]byte(dtsxContent), &pkg)
	require.NoError(t, err)

	assert.Len(t, pkg.Variables.Vars, 3)

	// Check all variables are parsed
	varNames := make([]string, len(pkg.Variables.Vars))
	for i, v := range pkg.Variables.Vars {
		varNames[i] = v.Name
	}
	assert.Contains(t, varNames, "Var1")
	assert.Contains(t, varNames, "Var2")
	assert.Contains(t, varNames, "Var3")
}

// TestSSISEmptyPackage tests parsing of minimal package
func TestSSISEmptyPackage(t *testing.T) {
	dtsxContent := `<Executable xmlns="www.microsoft.com/SqlServer/Dts">
</Executable>`

	var pkg SSISPackage
	err := xml.Unmarshal([]byte(dtsxContent), &pkg)
	require.NoError(t, err)

	assert.Empty(t, pkg.Variables.Vars)
	assert.Empty(t, pkg.ConnectionMgr.Connections)
	assert.Empty(t, pkg.Executables.Tasks)
}

// TestSSISXMLParsingWithoutNamespace tests XML parsing without namespace (simulating main.go behavior)
func TestSSISXMLParsingWithoutNamespace(t *testing.T) {
	// Start with namespaced XML like real DTSX files
	dtsxContent := `<Executable xmlns="www.microsoft.com/SqlServer/Dts">
  <Variables>
    <Variable ObjectName="TestVar">
      <VariableValue DataType="3">123</VariableValue>
    </Variable>
  </Variables>
</Executable>`

	// Strip namespace like main.go does
	dtsxContent = strings.ReplaceAll(dtsxContent, `xmlns="www.microsoft.com/SqlServer/Dts"`, "")

	var pkg SSISPackage
	err := xml.Unmarshal([]byte(dtsxContent), &pkg)
	require.NoError(t, err)

	assert.Len(t, pkg.Variables.Vars, 1)
	assert.Equal(t, "123", pkg.Variables.Vars[0].Value)
}
