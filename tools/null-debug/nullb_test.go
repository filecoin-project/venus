package main

import (
	"fmt"
	"testing"
)

// Test cases come from https://www.csun.edu/~hcmth031/tlroh.pdf.
func TestCalcC(t *testing.T) {
	results := make(map[int]map[int]int64)
	for n := 0; n <= 8; n++ {
		results[n] = make(map[int]int64)
//		fmt.Printf("\n\n****\n")
		for k := 0; k <= n; k++ {
			results[n][k] = c(n, k, 3).Int64()
//			fmt.Printf("testing -- n: %d k:%d c:%d\n", n, k, results[n][k])
		}
	}
	expected8 := map[int]int64{
		0: int64(1),
		1: int64(8),
		2: int64(28),
		3: int64(56),
		4: int64(65),
		5: int64(40),
		6: int64(10),
		7: int64(0),
		8: int64(0),
	}
	for k, v := range results[8] {
		if expected8[k] != v {
			t.Errorf("expected: %d, actual: %d", expected8[k], v)
		}
	}

}

func TestCalcProbNoMoreThanX(t *testing.T) {
	v := probNoMoreThanX(5, 80)
	fmt.Printf("5 of 80: %v\n", v)

/*	v = probNoMoreThanX(4, 100)
	fmt.Printf("4 of 100: %v\n", v)

	v = probNoMoreThanX(3, 100)
	fmt.Printf("3 of 100: %v\n", v)

	v = probNoMoreThanX(6, 100)
	fmt.Printf("6 of 100: %v\n", v)

	v = probNoMoreThanX(3, 40)
	fmt.Printf("3 of 40: %v\n", v)*/

	v = probNoMoreThanX(9, 1500)
	fmt.Printf("9 of 1500: %v\n", v)						

	v = probNoMoreThanX(8, 1500)
	fmt.Printf("8 of 1500: %v\n", v)					

	v = probNoMoreThanX(7, 1500)
	fmt.Printf("7 of 1500: %v\n", v)				

	v = probNoMoreThanX(6, 1500)
	fmt.Printf("6 of 1500: %v\n", v)			

	v = probNoMoreThanX(5, 1500)
	fmt.Printf("5 of 1500: %v\n", v)		

	v = probNoMoreThanX(4, 1500)
	fmt.Printf("4 of 1500: %v\n", v)	
	
	v = probNoMoreThanX(3, 1500)
	fmt.Printf("3 of 1500: %v\n", v)

	v = probNoMoreThanX(2, 1500)
	fmt.Printf("2 of 1500: %v\n", v)		

//	v = probNoMoreThanX(6, 7000)
//	fmt.Printf("6 of 7000: %v\n", v)	
	
/*
	v = probNoMoreThanX(11, 1000)
	fmt.Printf("9 of 1000: %v\n", v)

	v = probNoMoreThanX(6, 1000)
	fmt.Printf("6 of 1000: %v\n", v)

	v = probNoMoreThanX(5, 1000)
	fmt.Printf("5 of 1000: %v\n", v)

	v = probNoMoreThanX(4, 1000)
	fmt.Printf("4 of 1000: %v\n", v)

	v = probNoMoreThanX(3, 1000)
	fmt.Printf("3 of 1000: %v\n", v)						

	v = probNoMoreThanX(2, 1000)
	fmt.Printf("2 of 1000: %v\n", v)					*/
}
