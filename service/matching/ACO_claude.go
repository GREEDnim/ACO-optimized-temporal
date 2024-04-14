package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

type AllocatedTaskInfo struct {
	Data         *TaskInfo
	TaskID       int64
	FacetsValue  FacetsValue
	dependantTaskId protoimpl.UniqueId
}

type FacetsValue struct {
	Bandwidth  float64
	Latency    float64
	CPU        float64
	RetryLimit int
	Timeout    float64
}

type TaskInfo struct {
	// Add any other relevant fields here
}

type RouteDiscovery struct {
	tasks            []*AllocatedTaskInfo
	pheromones       [][]float64
	distances        [][]float64
	alpha            float64
	beta             float64
	initialPheromone float64
	remainingFactor  float64
	q                float64
	randomFactor     float64
	maxIterations    int
	numberOfCities   int
	numberOfAnts     int
	antFactor        float64
	ants             []*Ant
	random           *rand.Rand
	probabilities    []float64
	currentIndex     int
	bestTourOrder    []int64
	bestTourLength   float64
}

type Ant struct {
	trail        []int64
	visited      []bool
	trailSize    int
	trailLength  float64
	tourLength   float64
	facetsValues FacetsValue
}

func NewRouteDiscovery(tasks []*AllocatedTaskInfo) *RouteDiscovery {
	rd := &RouteDiscovery{
		tasks:            tasks,
		initialPheromone: 1.0,
		alpha:            1.0,
		beta:             5.0,
		remainingFactor:  0.5,
		q:                500.0,
		antFactor:        0.8,
		randomFactor:     0.01,
		maxIterations:    1000,
		random:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	rd.numberOfCities = rd.getTotalCities()
	rd.numberOfAnts = rd.numberOfCities
	rd.pheromones = make([][]float64, rd.numberOfCities)
	for i := range rd.pheromones {
		rd.pheromones[i] = make([]float64, rd.numberOfCities)
	}
	rd.distances = make([][]float64, rd.numberOfCities)
	for i := range rd.distances {
		rd.distances[i] = make([]float64, rd.numberOfCities)
	}
	rd.probabilities = make([]float64, rd.numberOfCities)

	rd.generateDistanceMatrix()
	rd.clearTrails()

	return rd
}

func (rd *RouteDiscovery) getTotalCities() int {
	citySet := make(map[int64]bool)
	for _, task := range rd.tasks {
		citySet[task.TaskID] = true
	}
	return len(citySet)
}

func (rd *RouteDiscovery) generateDistanceMatrix() {
	for i := 0; i < rd.numberOfCities; i++ {
		for j := 0; j < rd.numberOfCities; j++ {
			if i == j {
				rd.distances[i][j] = 0.0
			} else {
				rd.distances[i][j] = rd.tasks[i].FacetsValue.Latency
			}
		}
	}
}

func (rd *RouteDiscovery) InitiateOptimization() {
	rd.setupAnts()
	rd.clearTrails()
	for i := 0; i < rd.maxIterations; i++ {
		rd.moveAnts()
		rd.updateTrails()
		rd.updateBest()
	}
}

func (rd *RouteDiscovery) getPriorityMap() map[int64]int {
	priorityMap := make(map[int64]int)
	priority := 1
	for _, taskID := range rd.bestTourOrder {
		priorityMap[taskID] = priority
		priority++
	}
	return priorityMap
}

func (rd *RouteDiscovery) updateBest() {
	if rd.bestTourOrder == nil {
		rd.bestTourOrder = rd.ants[0].trail
		rd.bestTourLength = rd.ants[0].tourLength
	}

	for _, ant := range rd.ants {
		if ant.tourLength < rd.bestTourLength {
			rd.bestTourLength = ant.tourLength
			rd.bestTourOrder = make([]int64, len(ant.trail))
			copy(rd.bestTourOrder, ant.trail)
		}
	}
}

func (rd *RouteDiscovery) updateTrails() {
	for i := 0; i < rd.numberOfCities; i++ {
		for j := 0; j < rd.numberOfCities; j++ {
			rd.pheromones[i][j] *= rd.remainingFactor
		}
	}

	for _, ant := range rd.ants {
		contribution := rd.q / ant.tourLength
		for i := 0; i < rd.numberOfCities-1; i++ {
			rd.pheromones[ant.trail[i]][ant.trail[i+1]] += contribution
		}
		rd.pheromones[ant.trail[rd.numberOfCities-1]][ant.trail[0]] += contribution
	}
}

func (rd *RouteDiscovery) moveAnts() {
	for i := rd.currentIndex; i < rd.numberOfCities-1; i++ {
		for _, ant := range rd.ants {
			ant.visitCity(rd.currentIndex, rd.selectNextCity(ant))
		}
		rd.currentIndex++
	}
}

func (rd *RouteDiscovery) selectNextCity(ant *Ant) int64 {
	t := rd.random.Intn(rd.numberOfCities - rd.currentIndex)
	if rd.random.Float64() < rd.randomFactor {
		for i := 0; i < rd.numberOfCities; i++ {
			if i == t && !ant.visited[i] {
				return int64(i)
			}
		}
	}

	rd.calculateProbabilities(ant)
	r := rd.random.Float64()
	total := 0.0
	for i := 0; i < rd.numberOfCities; i++ {
		total += rd.probabilities[i]
		if total >= r {
			return int64(i)
		}
	}

	panic("There are no other cities")
}

func (rd *RouteDiscovery) calculateProbabilities(ant *Ant) {
	i := ant.trail[rd.currentIndex]
	pheromone := 0.0
	for l := 0; l < rd.numberOfCities; l++ {
		if !ant.visited[l] {
			pheromone += math.Pow(rd.pheromones[i][l], rd.alpha) * math.Pow(1.0/rd.distances[i][l]*ant.facetsValues.CPU, rd.beta)
		}
	}

	for j := 0; j < rd.numberOfCities; j++ {
		if ant.visited[j] {
			rd.probabilities[j] = 0.0
		} else {
			numerator := math.Pow(rd.pheromones[i][j], rd.alpha) * math.Pow(1.0/rd.distances[i][j]*ant.facetsValues.CPU, rd.beta)
			rd.probabilities[j] = numerator / pheromone
		}
	}
}

func (rd *RouteDiscovery) clearTrails() {
	for i := 0; i < rd.numberOfCities; i++ {
		for j := 0; j < rd.numberOfCities; j++ {
			rd.pheromones[i][j] = rd.initialPheromone
		}
	}
}

func (rd *RouteDiscovery) setupAnts() {
	rd.ants = make([]*Ant, rd.numberOfAnts)
	for i := 0; i < rd.numberOfAnts; i++ {
		rd.ants[i] = newAnt(rd.numberOfCities, rd.tasks[i].FacetsValue)
	}
	fmt.Println("Number of cities:", rd.numberOfCities)
	for _, ant := range rd.ants {
		ant.clear()
		ant.visitCity(-1, int64(rd.random.Intn(rd.numberOfCities)))
	}
	rd.currentIndex = 0
	fmt.Println("Number of ants:", rd.numberOfAnts)
	fmt.Println("Ants:", rd.ants)
}

func newAnt(trailSize int, facetsValues FacetsValue) *Ant {
	return &Ant{
	trail:        make([]int64, trailSize),
	visited:      make([]bool, trailSize),
	trailSize:    trailSize,
	facetsValues: facetsValues,
	}
}

func (ant *Ant) visitCity(currentIndex int, city int64) {
	ant.trail[currentIndex+1] = city
	ant.visited[city] = true
}

func (ant *Ant) visited(i int) bool {
	return ant.visited[i]
}


func (ant *Ant) calculateTourLength(distances [][]float64) {
	ant.tourLength = distances[ant.trail[ant.trailSize-1]][ant.trail[0]]
	for i := 0; i < ant.trailSize-1; i++ {
		ant.tourLength += distances[ant.trail[i]][ant.trail[i+1]]
	}
}

func (ant *Ant) clear() {
	for i := 0; i < ant.trailSize; i++ {
		ant.visited[i] = false
	}
}