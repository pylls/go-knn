/*
Package main implements the kNN website fingerprinting attack by Wang et al.,
providing support for folders of work and multiple cores.
*/
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"path"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"
)

type metrics struct { // see http://www.cs.kau.se/pulls/hot/measurements/
	tp  int // true positive
	fpp int // false-positive-to-positive
	fnp int // false-negative-to-positive
	fn  int // false negative
	tn  int // true negative
}

const (
	// FeatNum is the number of extracted features to consider in Wa-kNN.
	FeatNum int = 1225
	// FeatureSuffix is the suffix of files containing features.
	FeatureSuffix = ".feat"
	// RecoPointsNum is the number of neighbours for distance learning.
	RecoPointsNum int = 5
)

var (
	// datset
	sites     = flag.Int("sites", 0, "number of sites")
	instances = flag.Int("instances", 0, "number of instances")
	open      = flag.Int("open", 0, "number of open-world sites")
	roffset   = flag.Int("roffset", 0, "the offset to read monitored sites from")

	// Wa-kNN-related
	weightRounds = flag.Int("r", 2500, "rounds for WLLCC weight learning in kNN")
	wKmin        = flag.Int("wKmin", 1, "the smallest k to test for with Wa-kNN")
	wKmax        = flag.Int("wKmax", 2, "the biggest k to test for with Wa-kNN")
	wKstep       = flag.Int("wKstep", 1, "the step size between wKmin and wKmax")

	// experiment tweaks
	workerFactor = flag.Int("f", 1,
		"the factor to multiply NumCPU with for creating workers")
	folds = flag.Int("folds", 10,
		"we perform k-fold cross-validation")
	verboseOutput = flag.Bool("verbose", true, "print detailed result output")
	quiet         = flag.Bool("quiet", false,
		"don't print detailed progress (useful for not spamming docker log)")

	datadir = ""
)

func main() {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()
	if *sites == 0 || *instances == 0 {
		log.Println("missing sites and/or instances argument")
		flag.Usage()
		return
	}
	if len(flag.Args()) == 0 {
		log.Fatal("need to specify data dir")
	}
	datadir = flag.Arg(0)

	// can traces be split into k samples?
	if *instances%*folds != 0 || *open%*folds != 0 {
		log.Fatalf("error: k (%d) has to fold instances (%d) and open (%d) evenly",
			*folds, *instances, *open)
	}

	// find subfolders, do run for all of them, then print results
	var subfold []string
	files, err := ioutil.ReadDir(datadir)
	if err != nil {
		log.Fatalf("failed to read data folder (%s)", err)
	}
	for _, f := range files {
		if f.IsDir() {
			subfold = append(subfold, f.Name())
		}
	}
	if len(subfold) == 0 { // no subfolder, assume data folder is full of work
		subfold = append(subfold, datadir)
	}
	sort.Strings(subfold) // for deterministic output
	log.Printf("found %d folder(s) with work", len(subfold))

	// results is work -> map["attack"] -> [folds]metrics
	results := make([]map[string][]metrics, len(subfold))
	// allWeights is work -> fold -> features -> weight
	allWeights := make([][][]float64, len(subfold))
	for sub := 0; sub < len(subfold); sub++ {
		results[sub] = make(map[string][]metrics)
		log.Printf("starting with work %s", subfold[sub])

		// read cells from datadir
		log.Println("\tattempting to read WF features...")
		var feat, openfeat [][]float64
		if subfold[sub] == datadir && len(subfold) == 1 { // likely no subfolders
			feat, openfeat = readFeatures(subfold[sub])
		} else { // need full path
			feat, openfeat = readFeatures(path.Join(datadir, subfold[sub]))
		}

		log.Printf("\tread %d sites with %d instances (in total %d)",
			*sites, *instances, len(feat))
		log.Printf("\tread %d sites for open world", len(openfeat))

		testPerFold := (*sites**instances + *open) / *folds

		// calculate global weights for kNN in parallel (they don't change in folds)
		globalWeights := make([][]float64, *folds)
		wg := new(sync.WaitGroup)
		for fold := 0; fold < *folds; fold++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				globalWeights[i] = wllcc(feat, openfeat, i)
			}(fold)
		}
		wg.Wait()
		log.Printf("\tdetermined global kNN-weights for all folds")

		for fold := 0; fold < *folds; fold++ {
			log.Printf("\tstarting fold %d/%d", fold+1, *folds)

			// start workers
			workerIn := make(chan int)
			workerOut := make(chan map[string]metrics,
				(*sites**instances+*open) / *folds + 1000)
			wg := new(sync.WaitGroup)
			for i := 0; i < runtime.NumCPU()**workerFactor; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := range workerIn {
						workerOut <- test(j,
							fold, globalWeights[fold],
							feat, openfeat)
					}
				}()
			}

			// for each testing instance
			testing := 0
			for i := 0; i < *sites**instances+*open; i++ {
				if instanceForTesting(i, fold) {
					workerIn <- i
					testing++
					if !*quiet {
						fmt.Printf("\r\t\t\t\ttesting %d/%d (%d workers)",
							testing, testPerFold, runtime.NumCPU()**workerFactor)
					}
				}
			}
			if !*quiet {
				fmt.Println("")
			}

			close(workerIn)
			wg.Wait()
			close(workerOut)

			// save fold results
			for res := range workerOut {
				for attack, m := range res {
					_, exists := results[sub][attack]
					if !exists {
						results[sub][attack] = make([]metrics, *folds)
					}
					addResult(&results[sub][attack][fold], &m)
				}
			}
		}
		// save weights for all folds
		allWeights[sub] = globalWeights

	}

	// calculate results and store
	output := make(map[string]string)
	var attacks []string
	for attack := range results[0] {
		attacks = append(attacks, attack)
		output[attack] = "work,recall,precision,f1score,fpr,accuracy\n"
	}
	sort.Strings(attacks) // for deterministic output

	for i := 0; i < len(subfold); i++ {
		for attack, m := range results[i] {
			output[attack] += fmt.Sprintf("%s,%.3f,%.3f,%.3f,%.3f,%.3f\n",
				subfold[i], recall(m), precision(m), f1score(m), fpr(m), accuracy(m))
			if *verboseOutput {
				for j := 0; j < len(m); j++ {
					output[attack] += fmt.Sprintf("\ttp%d,fpp%d,fnp%d,fn%d,tn%d\n",
						m[j].tp, m[j].fpp, m[j].fnp, m[j].fn, m[j].tn)
				}
			}
		}
	}

	// CSV files for recall and precision
	generateCSV(recall,
		fmt.Sprintf("%dx%d+%d-%s.csv",
			*sites, *instances, *open, "recall"),
		results, attacks, subfold)
	generateCSV(precision,
		fmt.Sprintf("%dx%d+%d-%s.csv",
			*sites, *instances, *open, "precision"),
		results, attacks, subfold)

	// store a log to file of the complete run
	flog := fmt.Sprintf("%s: wfdns for %dx%d+%d\n\n",
		time.Now().String(), *sites, *instances, *open)
	for i := 0; i < len(attacks); i++ {
		log.Printf("%s attack", attacks[i])
		fmt.Printf("%s\n", output[attacks[i]])

		flog += fmt.Sprintf("%s attack\n%s\n", attacks[i], output[attacks[i]])
	}
	writeFile(flog,
		fmt.Sprintf("%dx%d+%d.log",
			*sites, *instances, *open))

	// write weights file
	wout := bytes.NewBufferString("work,fold") // ,f0,f1,....
	for i := 0; i < len(allWeights[0][0]); i++ {
		str2buf(fmt.Sprintf(",f%d", i+1), wout)
	}
	str2buf("\n", wout)
	for i := 0; i < len(allWeights); i++ {
		for j := 0; j < len(allWeights[i]); j++ {
			str2buf(fmt.Sprintf("%s,%d", subfold[i], j), wout)
			for k := 0; k < len(allWeights[i][j]); k++ {
				str2buf(fmt.Sprintf(",%s",
					strconv.FormatFloat(allWeights[i][j][k], 'f', -1, 64)), wout)
			}
			str2buf("\n", wout)
		}
	}
	writeFile(wout.String(), fmt.Sprintf("%dx%d+%d.weights",
		*sites, *instances, *open))
}

func test(i int, // test-specific
	fold int, globalWeight []float64, // fold-specific
	feat, openfeat [][]float64) (result map[string]metrics) {
	result = make(map[string]metrics)

	wKclasses, trueclass := classify(i, feat, openfeat,
		globalWeight, *wKmax, fold)

	for k := *wKmin; k <= *wKmax; k += *wKstep {
		n := fmt.Sprintf("k%s-", strconv.Itoa(k))
		result[n+"wf"] = getResult(getkNNClass(wKclasses, trueclass, k), trueclass)
	}

	return
}
