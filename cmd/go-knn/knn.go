package main

import (
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"path"
	"strconv"
	"strings"
)

type ignoreSite func(int) bool

func dist(from, to, weight []float64, presentFromFeat []int) (d float64) {
	for _, i := range presentFromFeat {
		if to[i] != -1 { // also present to?
			d += weight[i] * math.Abs(from[i]-to[i])
		}
	}
	return
}

func getMin(f []float64) (val float64, index int) {
	index = 0
	val = f[0]
	for i := 0; i < len(f); i++ {
		if f[i] < val {
			val = f[i]
			index = i
		}
	}
	return
}

func readFeatures(root string) (feat, openfeat [][]float64) {
	// flag all sites we read
	done := make(map[int]bool)

	// monitored sites
	for i := 0; i < *sites; i++ {
		site := *roffset + i + 1
		for j := 0; j < *instances; j++ {
			feat = append(feat,
				read(path.Join(root, strconv.Itoa(site)+"-"+strconv.Itoa(j)+FeatureSuffix)))
		}
		done[site] = true
	}

	// open sites, attempt to read *unmonitored number of sites from the
	// folder that we didn't already read
	files, err := ioutil.ReadDir(root)
	if err != nil {
		log.Fatalf("failed to read unmonitored folder (%s)", err)
	}
	for i := 0; i < len(files); i++ {
		// read site
		index := strings.Index(files[i].Name(), "-")
		if index == -1 || files[i].IsDir() {
			continue
		}
		s, err := strconv.Atoi(files[i].Name()[:index])
		if err != nil {
			continue
		}

		_, taken := done[s]
		if !taken {
			openfeat = append(openfeat,
				read(path.Join(root, files[i].Name())))
			done[s] = true
		}
		if len(done) >= *sites+*open {
			break
		}
		// break if done done
	}

	if len(done) < *sites+*open {
		log.Fatalf("failed to read %d open world sites", *open)
	}

	return
}

func read(filename string) (feat []float64) {
	d, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("failed to find file to read features for filename %s (%s)", filename, err)
	}

	// extract features
	for _, f := range strings.Split(string(d), " ") {
		if f == "'X'" {
			feat = append(feat, -1)
		} else if f != "" {
			feat = append(feat, parseFeatureString(f))
		}
	}
	return
}

func parseFeatureString(c string) float64 {
	val, err := strconv.ParseFloat(c, 64)
	if err != nil {
		panic(err)
	}
	if math.IsNaN(val) || math.IsInf(val, 1) || math.IsInf(val, 0) {
		// data is messy
		return -1
	}
	return val
}

func wllcc(feat, openfeat [][]float64, fold int) (weight []float64) {
	weight = make([]float64, FeatNum)
	// start with random weights between [0.5, 1.5]
	for i := 0; i < FeatNum; i++ {
		weight[i] = rand.Float64() + 0.5
	}

	distList := make([]float64, len(feat)+len(openfeat))
	recoGoodList := make([]int, RecoPointsNum)
	recoBadList := make([]int, RecoPointsNum)

	var ctr int
	sitePerm := rand.Perm(*sites) // random permutation of all sites
	// perform WeightRounds number of rounds of weight learning
	for round := 0; round < *weightRounds; round++ {
		// i is the instance of a monitored site used for distance calculations
		var i int
		for {
			// assume that we learn more from different sites than different
			// instances of the same site
			i = sitePerm[ctr%(len(sitePerm))]**instances + rand.Intn(*instances)
			ctr++
			if !instanceForTesting(i, fold) {
				break // only learn on training instances
			}
		}
		curSite := int(i / *instances)

		/*
		 distance calculation
		*/
		// optimization from @fowlslegs: determine present features
		presentFeat := make([]int, 0, FeatNum)
		for j := 0; j < FeatNum; j++ {
			if feat[i][j] != -1 {
				presentFeat = append(presentFeat, j)
			}
		}

		// calculate the distance to every other monitored instance
		for j := 0; j < len(feat); j++ {
			if instanceForTesting(j, fold) {
				distList[j] = math.MaxFloat64
			} else {
				distList[j] = dist(feat[i], feat[j], weight, presentFeat)
			}
		}
		// and the distance to all open sites
		for j := 0; j < len(openfeat); j++ {
			if instanceForTesting(j, fold) {
				distList[len(feat)+j] = math.MaxFloat64
			} else {
				distList[len(feat)+j] = dist(feat[i], openfeat[j], weight, presentFeat)
			}
		}

		/*
			weight recommendation
		*/
		var maxGoodDist float64
		// S_good = recoGoodList
		for j := 0; j < RecoPointsNum; j++ {
			_, minIndex := getMin(distList[curSite**instances : (curSite+1)**instances])
			// we have to add the off-set in the index above
			minIndex += curSite * *instances

			if distList[minIndex] > maxGoodDist {
				maxGoodDist = distList[minIndex]
			}

			// don't select the same instance again
			distList[minIndex] = math.MaxFloat64
			recoGoodList[j] = minIndex
		}

		// don't consider any instances for the current site in the future
		for j := 0; j < *instances; j++ {
			distList[curSite**instances+j] = math.MaxFloat64
		}

		// S_bad = recoBadList
		for j := 0; j < RecoPointsNum; j++ {
			_, minIndex := getMin(distList)
			// don't select the same instance again
			distList[minIndex] = math.MaxFloat64
			recoBadList[j] = minIndex
		}

		badList := make([]int, FeatNum)
		featDist := make([]float64, FeatNum)
		var minBadList int
		for j := 0; j < FeatNum; j++ {
			var countBad int

			// calculate maxgood for the feature (d_{f_i})
			var maxGood float64
			for k := 0; k < RecoPointsNum; k++ {
				n := math.Abs(feat[i][j] - feat[recoGoodList[k]][j])
				if feat[i][j] == -1 || feat[recoGoodList[k]][j] == -1 {
					n = 0
				}
				if n >= maxGood {
					maxGood = n
				}
			}

			// count bad distances (n_{bad_i})
			for k := 0; k < RecoPointsNum; k++ {
				var n float64
				if recoBadList[k] < len(feat) {
					// monitored
					n = math.Abs(feat[i][j] - feat[recoBadList[k]][j])
					if feat[i][j] == -1 || feat[recoBadList[k]][j] == -1 {
						n = 0
					}
				} else {
					// open
					n = math.Abs(feat[i][j] - openfeat[recoBadList[k]-len(feat)][j])
					if feat[i][j] == -1 || openfeat[recoBadList[k]-len(feat)][j] == -1 {
						n = 0
					}
				}

				if n <= maxGood {
					countBad++
				}
				// save distance for later
				featDist[j] += n
			}

			badList[j] = countBad
			if countBad < minBadList {
				minBadList = countBad
			}
		}

		/*
			weight adjustment
		*/
		// find out how poorly the current point is classified
		var distCountBad int
		for j := 0; j < RecoPointsNum; j++ {
			if recoBadList[j] < len(feat) &&
				dist(feat[i], feat[recoBadList[j]],
					weight, presentFeat) <= maxGoodDist {
				distCountBad++
			} else if recoBadList[j] >= len(feat) &&
				dist(feat[i], openfeat[recoBadList[j]-len(feat)],
					weight, presentFeat) <= maxGoodDist {
				distCountBad++
			}
		}

		for j := 0; j < FeatNum; j++ {
			// only adjust weight for non-min countBad features
			if badList[j] != minBadList {
				// reduce by weight * 0.01 * (n_{bad_i} / reco) * (1 + N_bad) / reco
				weight[j] -= weight[j] * 0.01 * (float64(badList[j]) / float64(RecoPointsNum)) * float64(1+distCountBad) / float64(RecoPointsNum)
			}
			// increase all weights by min(n_bad)
			weight[j] += float64(minBadList)
		}
	}

	return
}

func classify(test int, feat, openfeat [][]float64, weight []float64,
	neighbours, fold int) (classes []int, trueClass int) {
	// support classifying an open-world instance
	var testfeat []float64
	if test < len(feat) {
		testfeat = feat[test]
	} else {
		testfeat = openfeat[test-len(feat)]
	}

	// optimization from @fowlslegs: determine present features
	presentFeat := make([]int, 0, FeatNum)
	for i := 0; i < FeatNum; i++ {
		if testfeat[i] != -1 {
			presentFeat = append(presentFeat, i)
		}
	}

	distList := make([]float64, len(feat)+len(openfeat))
	for i := 0; i < len(feat); i++ {
		if instanceForTesting(i, fold) {
			distList[i] = math.MaxFloat64
		} else {
			// distance to all sites and their instances
			distList[i] = dist(testfeat, feat[i], weight, presentFeat)
		}
	}

	for i := 0; i < len(openfeat); i++ {
		if instanceForTesting(i, fold) {
			distList[len(feat)+i] = math.MaxFloat64
		} else {
			// distance to all open-world sites
			distList[len(feat)+i] = dist(testfeat, openfeat[i], weight, presentFeat)
		}
	}

	for i := 0; i < neighbours; i++ {
		_, index := getMin(distList)
		class := index / *instances
		if class > *sites {
			// we use the last class to represent all open-world sites
			class = *sites
		}
		classes = append(classes, class)

		distList[index] = math.MaxFloat64
	}

	trueClass = test / *instances
	if trueClass > *sites {
		trueClass = *sites
	}

	return
}

func getkNNClass(classes []int, trueclass, k int) (out int) {
	// classifier guesses unmonitored unless k closest classes agree on something
	out = *sites
	unmonitored := false
	for i := 0; i < k-1; i++ {
		if classes[i] != classes[i+1] {
			unmonitored = true
			break
		}
	}
	if !unmonitored {
		out = classes[0]
	}
	return
}
