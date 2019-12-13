// Copyright 2016 The ebakus/node Authors
// This file is part of ebakus/node.
//
// ebakus/node is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// ebakus/node is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with ebakus/node. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ebakus/node/accounts/abi/bind"
	"github.com/ebakus/node/cmd/utils"
	"github.com/ebakus/node/common/compiler"
	"github.com/ebakus/node/crypto"
	"github.com/ebakus/node/log"
	"gopkg.in/urfave/cli.v1"
)

const (
	commandHelperTemplate = `{{.Name}}{{if .Subcommands}} command{{end}}{{if .Flags}} [command options]{{end}} [arguments...]
{{if .Description}}{{.Description}}
{{end}}{{if .Subcommands}}
SUBCOMMANDS:
	{{range .Subcommands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
	{{end}}{{end}}{{if .Flags}}
OPTIONS:
{{range $.Flags}}{{"\t"}}{{.}}
{{end}}
{{end}}`
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	gitDate   = ""

	app *cli.App

	// Flags needed by abigen
	abiFlag = cli.StringFlag{
		Name:  "abi",
		Usage: "Path to the Ebakus contract ABI json to bind, - for STDIN",
	}
	binFlag = cli.StringFlag{
		Name:  "bin",
		Usage: "Path to the Ebakus contract bytecode (generate deploy method)",
	}
	typeFlag = cli.StringFlag{
		Name:  "type",
		Usage: "Struct name for the binding (default = package name)",
	}
	jsonFlag = cli.StringFlag{
		Name:  "combined-json",
		Usage: "Path to the combined-json file generated by compiler",
	}
	solFlag = cli.StringFlag{
		Name:  "sol",
		Usage: "Path to the Ebakus contract Solidity source to build and bind",
	}
	solcFlag = cli.StringFlag{
		Name:  "solc",
		Usage: "Solidity compiler to use if source builds are requested",
		Value: "solc",
	}
	vyFlag = cli.StringFlag{
		Name:  "vy",
		Usage: "Path to the Ebakus contract Vyper source to build and bind",
	}
	vyperFlag = cli.StringFlag{
		Name:  "vyper",
		Usage: "Vyper compiler to use if source builds are requested",
		Value: "vyper",
	}
	excFlag = cli.StringFlag{
		Name:  "exc",
		Usage: "Comma separated types to exclude from binding",
	}
	pkgFlag = cli.StringFlag{
		Name:  "pkg",
		Usage: "Package name to generate the binding into",
	}
	outFlag = cli.StringFlag{
		Name:  "out",
		Usage: "Output file for the generated binding (default = stdout)",
	}
	langFlag = cli.StringFlag{
		Name:  "lang",
		Usage: "Destination language for the bindings (go, java, objc)",
		Value: "go",
	}
)

func init() {
	app = utils.NewApp(gitCommit, gitDate, "ebakus checkpoint helper tool")
	app.Flags = []cli.Flag{
		abiFlag,
		binFlag,
		typeFlag,
		jsonFlag,
		solFlag,
		solcFlag,
		vyFlag,
		vyperFlag,
		excFlag,
		pkgFlag,
		outFlag,
		langFlag,
	}
	app.Action = utils.MigrateFlags(abigen)
	cli.CommandHelpTemplate = commandHelperTemplate
}

func abigen(c *cli.Context) error {
	utils.CheckExclusive(c, abiFlag, jsonFlag, solFlag, vyFlag) // Only one source can be selected.
	if c.GlobalString(pkgFlag.Name) == "" {
		utils.Fatalf("No destination package specified (--pkg)")
	}
	var lang bind.Lang
	switch c.GlobalString(langFlag.Name) {
	case "go":
		lang = bind.LangGo
	case "java":
		lang = bind.LangJava
	case "objc":
		lang = bind.LangObjC
		utils.Fatalf("Objc binding generation is uncompleted")
	default:
		utils.Fatalf("Unsupported destination language \"%s\" (--lang)", c.GlobalString(langFlag.Name))
	}
	// If the entire solidity code was specified, build and bind based on that
	var (
		abis  []string
		bins  []string
		types []string
		sigs  []map[string]string
		libs  = make(map[string]string)
	)
	if c.GlobalString(abiFlag.Name) != "" {
		// Load up the ABI, optional bytecode and type name from the parameters
		var (
			abi []byte
			err error
		)
		input := c.GlobalString(abiFlag.Name)
		if input == "-" {
			abi, err = ioutil.ReadAll(os.Stdin)
		} else {
			abi, err = ioutil.ReadFile(input)
		}
		if err != nil {
			utils.Fatalf("Failed to read input ABI: %v", err)
		}
		abis = append(abis, string(abi))

		var bin []byte
		if binFile := c.GlobalString(binFlag.Name); binFile != "" {
			if bin, err = ioutil.ReadFile(binFile); err != nil {
				utils.Fatalf("Failed to read input bytecode: %v", err)
			}
			if strings.Contains(string(bin), "//") {
				utils.Fatalf("Contract has additional library references, please use other mode(e.g. --combined-json) to catch library infos")
			}
		}
		bins = append(bins, string(bin))

		kind := c.GlobalString(typeFlag.Name)
		if kind == "" {
			kind = c.GlobalString(pkgFlag.Name)
		}
		types = append(types, kind)
	} else {
		// Generate the list of types to exclude from binding
		exclude := make(map[string]bool)
		for _, kind := range strings.Split(c.GlobalString(excFlag.Name), ",") {
			exclude[strings.ToLower(kind)] = true
		}
		var err error
		var contracts map[string]*compiler.Contract

		switch {
		case c.GlobalIsSet(solFlag.Name):
			contracts, err = compiler.CompileSolidity(c.GlobalString(solcFlag.Name), c.GlobalString(solFlag.Name))
			if err != nil {
				utils.Fatalf("Failed to build Solidity contract: %v", err)
			}
		case c.GlobalIsSet(vyFlag.Name):
			contracts, err = compiler.CompileVyper(c.GlobalString(vyperFlag.Name), c.GlobalString(vyFlag.Name))
			if err != nil {
				utils.Fatalf("Failed to build Vyper contract: %v", err)
			}
		case c.GlobalIsSet(jsonFlag.Name):
			jsonOutput, err := ioutil.ReadFile(c.GlobalString(jsonFlag.Name))
			if err != nil {
				utils.Fatalf("Failed to read combined-json from compiler: %v", err)
			}
			contracts, err = compiler.ParseCombinedJSON(jsonOutput, "", "", "", "")
			if err != nil {
				utils.Fatalf("Failed to read contract information from json output: %v", err)
			}
		}
		// Gather all non-excluded contract for binding
		for name, contract := range contracts {
			if exclude[strings.ToLower(name)] {
				continue
			}
			abi, err := json.Marshal(contract.Info.AbiDefinition) // Flatten the compiler parse
			if err != nil {
				utils.Fatalf("Failed to parse ABIs from compiler output: %v", err)
			}
			abis = append(abis, string(abi))
			bins = append(bins, contract.Code)
			sigs = append(sigs, contract.Hashes)
			nameParts := strings.Split(name, ":")
			types = append(types, nameParts[len(nameParts)-1])

			libPattern := crypto.Keccak256Hash([]byte(name)).String()[2:36]
			libs[libPattern] = nameParts[len(nameParts)-1]
		}
	}
	// Generate the contract binding
	code, err := bind.Bind(types, abis, bins, sigs, c.GlobalString(pkgFlag.Name), lang, libs)
	if err != nil {
		utils.Fatalf("Failed to generate ABI binding: %v", err)
	}
	// Either flush it out to a file or display on the standard output
	if !c.GlobalIsSet(outFlag.Name) {
		fmt.Printf("%s\n", code)
		return nil
	}
	if err := ioutil.WriteFile(c.GlobalString(outFlag.Name), []byte(code), 0600); err != nil {
		utils.Fatalf("Failed to write ABI binding: %v", err)
	}
	return nil
}

func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
