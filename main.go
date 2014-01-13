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
	pkgs            map[string]*build.Package
	ids             map[string]int
	networkPackages map[string]int
	nextId          int

	ignored = map[string]bool{
		"C": true,
	}
	ignoredPrefixes  []string
	includedPackages []string
	basePath         string

	ignoreStdlib     = flag.Bool("s", false, "ignore packages in the go standard library")
	ignorePrefixes   = flag.String("p", "", "a comma-separated list of prefixes to ignore")
	ignorePackages   = flag.String("i", "", "a comma-separated list of packages to ignore")
	includePackages  = flag.String("n", "", "a comma-separated list of packages to always include, even if ignored before")
	filterByBasePath = flag.Bool("b", false, "filer only for packages that are in the base path. other packages will be ignored except i they are in includePackages")
	subgraph         = flag.Bool("subgraph", false, "put graph into a subgraph box")
	networkSubgraphs = flag.Bool("network-subgraphs", false, "for each always included package, put an own external subgraph. requires subgraph to be set")
)

func main() {
	pkgs = make(map[string]*build.Package)
	ids = make(map[string]int)
	networkPackages = make(map[string]int)
	flag.Parse()

	args := flag.Args()

	if len(args) != 1 {
		log.Fatal("need one package name to process")
	}

	if *ignorePrefixes != "" {
		ignoredPrefixes = sanitizeCSV(*ignorePrefixes)
	}
	if *ignorePackages != "" {
		for _, p := range sanitizeCSV(*ignorePackages) {
			ignored[p] = true
		}
	}
	if *includePackages != "" {
		includedPackages = sanitizeCSV(*includePackages)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get cwd: %s", err)
	}
	if err := processPackage(cwd, args[0]); err != nil {
		log.Fatal(err)
	}

	fmt.Println("digraph godep {")

	if *subgraph && basePath != "" {
		printSubgraphHead(basePath)
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

		printNode(pkgName, color)

		// Don't render imports from packages in Goroot
		if pkg.Goroot {
			//continue
		}

		for _, imp := range pkg.Imports {
			impPkg := pkgs[imp]
			if impPkg == nil || isIgnored(impPkg) {
				continue
			}

			impId := getId(imp)
			printEdge(pkgId, impId)
		}

		// check if we need to build a network subgraph for this node later
		if *networkSubgraphs && hasPrefixes(pkg.ImportPath, includedPackages) {
			networkPackages[pkgName] = pkgId
		}
	}

	if *subgraph && basePath != "" {
		fmt.Println("}")
	}

	for pkgName, pkgId := range networkPackages {
		// make subgraph
		nameSplit := strings.Split(pkgName, "/")
		name := nameSplit[len(nameSplit)-1]
		printSubgraphHead(name)
		printNode(name, "paleturquoise")
		fmt.Println("}")

		// make edge
		printEdge(pkgId, getId(name))
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

	if *filterByBasePath && basePath == "" {
		// basePath has not been set yet
		// we assume that the first package we encouter is the root node
		// we assume that the base path is the root node's parent directory
		basePathSplit := strings.Split(pkg.ImportPath, "/")
		basePath = strings.Join(basePathSplit[0:len(basePathSplit)-1], "/")
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

func sanitizeCSV(csv string) []string {
  output := strings.Split(csv, ",")
  for i, v := range(output) {
    output[i] = strings.ToLower(strings.TrimSpace(v))
  }
  return output
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
			isNotOfBasepath(pkg.ImportPath, basePath))
}

func isNotOfBasepath(importPath, basePath string) bool {
	return basePath != "" && !strings.HasPrefix(importPath, basePath)
}

func printSubgraphHead(name string) {
	fmt.Printf("subgraph \"cluster%s\" {\n", name)
	fmt.Println("style=filled;")
	fmt.Println("color=lightgrey;")
	fmt.Printf("label=\"%s\"\n", name)
}

func printNode(name, color string) {
	fmt.Printf("%d [label=\"%s\" style=\"filled\" color=\"%s\"];\n", getId(name), name, color)
}

func printEdge(source, dest int) {
	fmt.Printf("%d -> %d;\n", source, dest)
}

func debug(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
}

func debugf(s string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, s, args...)
}
