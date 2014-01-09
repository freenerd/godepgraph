package main

import (
	"flag"
	"fmt"
	"go/build"
	"log"
	"os"
	"strings"
)

var (
	pkgs   map[string]*build.Package
	ids    map[string]int
	nextId int

	ignored = map[string]bool{
		"C": true,
	}
	ignoredPrefixes  []string
	includedPackages []string

	ignoreStdlib     = flag.Bool("s", false, "ignore packages in the go standard library")
	ignorePrefixes   = flag.String("p", "", "a comma-separated list of prefixes to ignore")
	ignorePackages   = flag.String("i", "", "a comma-separated list of packages to ignore")
	basePath         = flag.String("b", "", "the base path of the package. if this is set, all non-base path packages will be ignored")
	includePackages  = flag.String("n", "", "a comma-separated list of packages to always include, even if ignored before")
	subgraph         = flag.Bool("subgraph", false, "put graph into a subgraph box")
	networkSubgraphs = flag.Bool("network-subgraphs", false, "for each always included package, put an own external subgraph. requires subgraph to be set")
)

func main() {
	pkgs = make(map[string]*build.Package)
	ids = make(map[string]int)
	flag.Parse()

	args := flag.Args()

	if len(args) != 1 {
		log.Fatal("need one package name to process")
	}

	if *ignorePrefixes != "" {
		ignoredPrefixes = strings.Split(*ignorePrefixes, ",")
	}
	if *ignorePackages != "" {
		for _, p := range strings.Split(*ignorePackages, ",") {
			ignored[p] = true
		}
	}

	if *includePackages != "" {
		includedPackages = strings.Split(*includePackages, ",")
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get cwd: %s", err)
	}
	if err := processPackage(cwd, args[0]); err != nil {
		log.Fatal(err)
	}

	fmt.Println("digraph godep {")

	if *subgraph && *basePath != "" {
		fmt.Printf("subgraph \"cluster%s\" {\n", *basePath)
		fmt.Println("style=filled;")
		fmt.Println("color=lightgrey;")
		fmt.Printf("label=\"%s\"\n", *basePath)
	}

	for pkgName, pkg := range pkgs {
		pkgId := getId(pkgName)

		if isIgnored(pkg) {
			continue
		}

		var color string
		if pkg.Goroot {
			color = "palegreen"
		} else if len(pkg.CgoFiles) > 0 {
			color = "darkgoldenrod1"
		} else if hasPrefixes(pkg.ImportPath, includedPackages) {
			color = "violet"
		} else {
			color = "paleturquoise"
		}

		fmt.Printf("%d [label=\"%s\" style=\"filled\" color=\"%s\"];\n", pkgId, pkgName, color)

		// Don't render imports from packages in Goroot
		if pkg.Goroot {
			continue
		}

		for _, imp := range pkg.Imports {
			impPkg := pkgs[imp]
			if impPkg == nil || isIgnored(impPkg) {
				continue
			}

			impId := getId(imp)
			fmt.Printf("%d -> %d;\n", pkgId, impId)
		}
	}

	if *subgraph && *basePath != "" {
		fmt.Println("}")
	}

	fmt.Println("}")
}

func processPackage(root string, pkgName string) error {
	if ignored[pkgName] {
		return nil
	}

	pkg, err := build.Import(pkgName, root, 0)
	if err != nil {
		return fmt.Errorf("failed to import %s: %s", pkgName, err)
	}

	if isIgnored(pkg) {
		return nil
	}

	pkgs[pkg.ImportPath] = pkg

	// Don't worry about dependencies for stdlib packages
	if pkg.Goroot {
		return nil
	}

	for _, imp := range pkg.Imports {
		if _, ok := pkgs[imp]; !ok {
			if err := processPackage(root, imp); err != nil {
				return err
			}
		}
	}
	return nil
}

func getId(name string) int {
	id, ok := ids[name]
	if !ok {
		id = nextId
		nextId++
		ids[name] = id
	}
	return id
}

func hasPrefixes(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func isIgnored(pkg *build.Package) bool {
	return !hasPrefixes(pkg.ImportPath, includedPackages) &&
		(ignored[pkg.ImportPath] ||
			(pkg.Goroot && *ignoreStdlib) ||
			hasPrefixes(pkg.ImportPath, ignoredPrefixes) ||
			isNotOfBasepath(pkg.ImportPath, *basePath))
}

func isNotOfBasepath(importPath, basePath string) bool {
	return basePath != "" && !strings.HasPrefix(importPath, basePath)
}

func debug(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
}

func debugf(s string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, s, args...)
}
