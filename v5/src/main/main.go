package main

import (
	"fmt"
	"golang.org/x/image/bmp"
	_ "golang.org/x/image/bmp"
	"image"
	"image/color"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/tuneinsight/lattigo/v4/ckks"
	"github.com/tuneinsight/lattigo/v4/rlwe"
)

/*                     ******************** Using batch for decryption *********************            */

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func bmpRead(filename string) image.Image {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	return img
}

func bmpWrite(filename string, img image.Image) {
	outFile, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()

	err = bmp.Encode(outFile, img)
	if err != nil {
		log.Fatal(err)
	}
}

func pixelToArray(img image.Image) [][]float64 {
	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y

	ret := make([][]float64, height)
	for y := 0; y < height; y++ {
		ret[y] = make([]float64, width*3)
		for x := 0; x < width; x++ {
			c := color.GrayModel.Convert(img.At(x, y))
			gray, _, _, _ := c.RGBA()
			ret[y][x*3+0] = float64(gray >> 8)
			ret[y][x*3+1] = float64(gray >> 8)
			ret[y][x*3+2] = float64(gray >> 8)
		}
	}
	return ret
}

func arrayToImage(arr [][]float64, startX, startY, width, height int, img *image.RGBA) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(startX+x, startY+y, color.RGBA{
				R: uint8(arr[y][x*3]),
				G: uint8(arr[y][x*3+1]),
				B: uint8(arr[y][x*3+2]),
				A: 255,
			})
		}
	}
}

/*
// Represent every RGB in base 2
func representBase2(num int) []int {
	var powers []int
	i := 0
	for num > 0 {
		if num%2 == 1 {
			powers = append(powers, i)
		}
		num /= 2
		i++
	}
	return powers
}

*/

// remove used elements
func removeElement(cipherzeroList []*rlwe.Ciphertext, index int) []*rlwe.Ciphertext {
	if len(cipherzeroList) == 0 {
		return cipherzeroList // Return the empty slice
	}

	if index < 0 || index >= len(cipherzeroList) {
		return cipherzeroList
	}

	// Create a new slice with enough capacity to hold all elements minus one.
	newSlice := make([]*rlwe.Ciphertext, 0, len(cipherzeroList)-1)
	newSlice = append(newSlice, cipherzeroList[:index]...)   // Append part before the index
	newSlice = append(newSlice, cipherzeroList[index+1:]...) // Append part after the index
	return newSlice
}

func encryptRGBList(params ckks.Parameters, secretKey rlwe.SecretKey) ([]*rlwe.Ciphertext, []*rlwe.Ciphertext) {
	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, secretKey)
	// The list that needs to be encrypted
	//plainList := []float64{1.0, 2.0, 4.0, 8.0, 16.0, 32.0, 64.0, 128.0, 0.0}
	var plainList []float64
	for i := 0; i <= 255; i++ {
		plainList = append(plainList, float64(i))
	}
	var zeroList []float64
	for i := 0; i < 10000; i++ {
		zeroList = append(zeroList, 0.0)
	}

	ciphertextList := make([]*rlwe.Ciphertext, len(plainList))
	for i, plain := range plainList {
		plaintext := ckks.NewPlaintext(params, params.MaxLevel())
		// Wrap the single float64 value in a slice of float64
		encoder.Encode([]float64{plain}, plaintext, params.LogSlots())
		ciphertext := encryptor.EncryptNew(plaintext)
		ciphertextList[i] = ciphertext
	}

	cipherzeroList := make([]*rlwe.Ciphertext, len(zeroList))
	for i, zero := range zeroList {
		plaintext1 := ckks.NewPlaintext(params, params.MaxLevel())
		// Wrap the single float64 value in a slice of float64
		encoder.Encode([]float64{zero}, plaintext1, params.LogSlots())
		ciphertext1 := encryptor.EncryptNew(plaintext1)
		cipherzeroList[i] = ciphertext1
	}

	return ciphertextList, cipherzeroList
}

func AddEnc(params ckks.Parameters, ciphertextList []*rlwe.Ciphertext, cipherzeroList []*rlwe.Ciphertext, publicKey rlwe.PublicKey, img image.Image, rKey *rlwe.RelinearizationKey) (pixelCipherText [][]*rlwe.Ciphertext) {
	parray := pixelToArray(img)
	height := len(parray)
	width := len(parray[0]) / 3
	pixelCipherText = make([][]*rlwe.Ciphertext, height)
	for i := 0; i < height; i++ {
		pixelCipherText[i] = make([]*rlwe.Ciphertext, width*3)
	}
	evaluator := ckks.NewEvaluator(params, rlwe.EvaluationKey{Rlk: rKey})

	for i := 0; i < height; i++ {
		for j := 0; j < width*3; j++ {
			rando := rand.Intn(9999)
			pixelCipherText[i][j] = evaluator.AddNew(ciphertextList[int(parray[i][j])], cipherzeroList[rando])
		}
	}

	return
}

func main() {
	start := time.Now().UnixMicro()
	img := bmpRead("/Users/sueyang/Desktop/Code/Single_key_CKKS/v4/src/main/linux100x100.bmp")
	end := time.Now().UnixMicro()
	fmt.Printf("ScanPic		: %8d μs\n", end-start)
	fmt.Printf("Resolution : %4d x%4d\n", img.Bounds().Max.X, img.Bounds().Max.Y)

	start = time.Now().UnixMicro()
	// Adjust parameters
	//params, err := ckks.NewParametersFromLiteral(ckks.PN14QP438)
	params, err := ckks.NewParametersFromLiteral(ckks.PN12QP109)
	check(err)
	secretKey, publicKey := ckks.NewKeyGenerator(params).GenKeyPair()
	rKey := ckks.NewKeyGenerator(params).GenRelinearizationKey(secretKey, 1)
	end = time.Now().UnixMicro()

	fmt.Printf("Setup		: %8d μs\n", end-start)

	start = time.Now().UnixMicro()
	ciphertextList, cipherzeroList := encryptRGBList(params, *secretKey)
	end = time.Now().UnixMicro()
	//fmt.Println(len(cipherzeroList))

	fmt.Printf("cache	: %8d μs\n", end-start)

	start = time.Now().UnixMicro()
	grayBody := AddEnc(params, ciphertextList, cipherzeroList, *publicKey, img, rKey)
	end = time.Now().UnixMicro()
	durationMicroseconds := end - start
	durationSeconds := float64(durationMicroseconds) / 1e6
	fmt.Printf("Homomorphic Addition	: %8.6f s\n", durationSeconds)

	//start = time.Now().UnixMicro()
	/*
		decbody := make([][][]float64, img.Bounds().Max.Y)
		for i := 0; i < img.Bounds().Max.Y; i++ {
			decbody[i] = make([][]float64, img.Bounds().Max.X)
			for j := 0; j < img.Bounds().Max.X; j++ {
				decbody[i][j] = make([]float64, 3)
			}
		}

		for i := 0; i < img.Bounds().Max.Y; i++ {
			for j := 0; j < img.Bounds().Max.X; j++ {
				for k := 0; k < 3; k++ {
					encoder := ckks.NewEncoder(params)
					decryptor := ckks.NewDecryptor(params, secretKey)
					decValues := encoder.Decode(decryptor.DecryptNew(grayBody[i][j*3+k]), params.LogSlots())
					decbody[i][j][k] = real(decValues[0])
				}
			}
		}

	*/

	height := img.Bounds().Max.Y
	width := img.Bounds().Max.X

	// Define the block size
	blockSize := 50
	// Create a new RGBA image to store the processed image
	processedImg := image.NewRGBA(image.Rect(0, 0, width, height))
	start = time.Now().UnixMicro()

	for startY := 0; startY < height; startY += blockSize {
		for startX := 0; startX < width; startX += blockSize {
			blockHeight := min(blockSize, height-startY)
			blockWidth := min(blockSize, width-startX)

			decbody := make([][][]float64, blockHeight)
			for i := 0; i < blockHeight; i++ {
				decbody[i] = make([][]float64, blockWidth)
				for j := 0; j < blockWidth; j++ {
					decbody[i][j] = make([]float64, 3)
					for k := 0; k < 3; k++ {
						encoder := ckks.NewEncoder(params)
						decryptor := ckks.NewDecryptor(params, secretKey)

						// Note that grayBody indices should be adjusted based on the current block's position
						globalY := startY + i
						globalX := startX + j
						decValues := encoder.Decode(decryptor.DecryptNew(grayBody[globalY][globalX*3+k]), params.LogSlots())

						decbody[i][j][k] = real(decValues[0])
					}
				}
			}

			// 3D to 2D conversion
			decbody2d := make([][]float64, blockHeight)
			for i := 0; i < blockHeight; i++ {
				decbody2d[i] = make([]float64, blockWidth*3)
				for j := 0; j < blockWidth; j++ {
					for k := 0; k < 3; k++ {
						decbody2d[i][j*3+k] = decbody[i][j][k]
					}
				}
			}

			arrayToImage(decbody2d, startX, startY, blockWidth, blockHeight, processedImg)
		}
	}

	end = time.Now().UnixMicro()
	durationMicroseconds = end - start
	durationSeconds = float64(durationMicroseconds) / 1e6
	fmt.Printf("Decryption: %8.6f s\n", durationSeconds)

	bmpWrite("50x50_2.bmp", processedImg)
	bmpWrite("50x50_3.bmp", img)

	afterPixel1 := convertToGray(bmpRead("50x50_2.bmp"))
	afterPixel2 := convertToGray(bmpRead("50x50_3.bmp"))

	if compareImages(afterPixel1, afterPixel2) {
		fmt.Println("50x50_2.bmp and 50x50_3.bmp are similar")
	} else {
		fmt.Println("50x50_2.bmp and 50x50_3.bmp are not similar")
	}
}

func convertToGray(img image.Image) *image.Gray {
	bounds := img.Bounds()
	grayImg := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			grayImg.Set(x, y, img.At(x, y))
		}
	}

	return grayImg
}

func compareImages(img1, img2 *image.Gray) bool {
	if img1.Bounds() != img2.Bounds() {
		return false
	}

	for i := 0; i < img1.Bounds().Max.Y; i++ {
		for j := 0; j < img1.Bounds().Max.X; j++ {
			if abs(int(img1.GrayAt(j, i).Y)-int(img2.GrayAt(j, i).Y)) > 1 {
				return false
			}
		}
	}

	return true
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
