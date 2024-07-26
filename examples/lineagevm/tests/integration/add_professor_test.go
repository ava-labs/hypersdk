package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/lineagevm/actions"
	"github.com/ava-labs/hypersdk/examples/lineagevm/storage"
)

const (
	// First sample professor
	nameOne       = "Leonhard Euler"
	yearOne       = uint16(1726)
	universityOne = "Universitat Basel"

	// Second sample professor
	nameTwo       = "Joseph Louis Lagrange"
	yearTwo       = uint16(1754)
	universityTwo = "Universita di Torino"

	// Third sample professor
	nameThree       = "Johann Friedrich Hennert"
	yearThree       = uint16(1766)
	universityThree = "Konigliche Akademie der Wissenschaften zu Berlin"

	// Fourth sample professor
	nameFour       = "Jean-Baptiste Joseph Fourier"
	yearFour       = uint16(1817)
	universityFour = "Ecole Normale Superieure"

	// Fifth sample professor
	nameFive       = "Simeon Denis Poisson"
	yearFive       = uint16(1800)
	universityFive = "Ecole Polytechnique"

	// Sixth sample professor
	nameSix       = "Gustav Peter Lejeune Dirichlet"
	yearSix       = uint16(1827)
	universitySix = "Rheinische Friedrich-Wilhelms-Universitat Bonn"
)

func TestAddProfessor(t *testing.T) {
	prep := prepare(t)

	parser, err := prep.instance.lcli.Parser(context.Background())
	require.NoError(t, err)

	submit, _, _, err := prep.instance.cli.GenerateTransaction(
		context.Background(),
		parser,
		[]chain.Action{
			&actions.AddProfessor{
				Name:       nameOne,
				Year:       yearOne,
				University: universityOne,
			},
		},
		factory,
	)

	require.NoError(t, err)
	require.NoError(t, submit(context.Background()))

	results := expectBlk(prep.instance)(false)
	require.Len(t, results, 1)
	require.True(t, results[0].Success)

	professorID := string(results[0].Outputs[0][0])

	nameFromChainOne, yearFromChainOne, universityFromChainOne, studentsFromChainOne, err := prep.instance.lcli.GetProfessorDetails(context.Background(), professorID)

	require.NoError(t, err)

	require.NoError(t, err)
	// Check that action one worked
	require.Equal(t, nameOne, nameFromChainOne)
	require.Equal(t, yearOne, yearFromChainOne)
	require.Equal(t, universityOne, universityFromChainOne)
	require.Equal(t, []codec.Address{}, studentsFromChainOne)
}

// Adds all professors to database
// Passes if all professors were added and their information is correct
func TestMultipleProfessors(t *testing.T) {
	prep := prepare(t)

	parser, err := prep.instance.lcli.Parser(context.Background())
	require.NoError(t, err)

	submit, _, _, err := prep.instance.cli.GenerateTransaction(
		context.Background(),
		parser,
		[]chain.Action{
			&actions.AddProfessor{
				Name:       nameOne,
				Year:       yearOne,
				University: universityOne,
			},
			&actions.AddProfessor{
				Name:       nameTwo,
				Year:       yearTwo,
				University: universityTwo,
			},
			&actions.AddProfessor{
				Name:       nameThree,
				Year:       yearThree,
				University: universityThree,
			},
			&actions.AddProfessor{
				Name:       nameFour,
				Year:       yearFour,
				University: universityFour,
			},
			&actions.AddProfessor{
				Name:       nameFive,
				Year:       yearFive,
				University: universityFive,
			},
			&actions.AddProfessor{
				Name:       nameSix,
				Year:       yearSix,
				University: universitySix,
			},
		},
		factory,
	)

	require.NoError(t, err)
	require.NoError(t, submit(context.Background()))

	results := expectBlk(prep.instance)(false)
	require.Len(t, results, 1)
	require.True(t, results[0].Success)

	professorOneID := string(results[0].Outputs[0][0])
	professorTwoID := string(results[0].Outputs[1][0])
	professorThreeID := string(results[0].Outputs[2][0])
	professorFourID := string(results[0].Outputs[3][0])
	professorFiveID := string(results[0].Outputs[4][0])
	professorSixID := string(results[0].Outputs[5][0])

	nameFromChainOne, yearFromChainOne, universityFromChainOne, studentsFromChainOne, err := prep.instance.lcli.GetProfessorDetails(context.Background(), professorOneID)

	require.NoError(t, err)

	nameFromChainTwo, yearFromChainTwo, universityFromChainTwo, studentsFromChainTwo, err := prep.instance.lcli.GetProfessorDetails(context.Background(), professorTwoID)

	require.NoError(t, err)

	nameFromChainThree, yearFromChainThree, universityFromChainThree, studentsFromChainThree, err := prep.instance.lcli.GetProfessorDetails(context.Background(), professorThreeID)

	require.NoError(t, err)

	nameFromChainFour, yearFromChainFour, universityFromChainFour, studentsFromChainFour, err := prep.instance.lcli.GetProfessorDetails(context.Background(), professorFourID)

	require.NoError(t, err)

	nameFromChainFive, yearFromChainFive, universityFromChainFive, studentsFromChainFive, err := prep.instance.lcli.GetProfessorDetails(context.Background(), professorFiveID)

	require.NoError(t, err)

	nameFromChainSix, yearFromChainSix, universityFromChainSix, studentsFromChainSix, err := prep.instance.lcli.GetProfessorDetails(context.Background(), professorSixID)

	require.NoError(t, err)

	// Check that action one worked
	require.Equal(t, nameOne, nameFromChainOne)
	require.Equal(t, yearOne, yearFromChainOne)
	require.Equal(t, universityOne, universityFromChainOne)
	require.Equal(t, []codec.Address{}, studentsFromChainOne)

	// Check that action two worked
	require.Equal(t, nameTwo, nameFromChainTwo)
	require.Equal(t, yearTwo, yearFromChainTwo)
	require.Equal(t, universityTwo, universityFromChainTwo)
	require.Equal(t, []codec.Address{}, studentsFromChainTwo)

	// Check that action three worked
	require.Equal(t, nameThree, nameFromChainThree)
	require.Equal(t, yearThree, yearFromChainThree)
	require.Equal(t, universityThree, universityFromChainThree)
	require.Equal(t, []codec.Address{}, studentsFromChainThree)

	// Check that action four worked
	require.Equal(t, nameFour, nameFromChainFour)
	require.Equal(t, yearFour, yearFromChainFour)
	require.Equal(t, universityFour, universityFromChainFour)
	require.Equal(t, []codec.Address{}, studentsFromChainFour)

	// Check that action five worked
	require.Equal(t, nameFive, nameFromChainFive)
	require.Equal(t, yearFive, yearFromChainFive)
	require.Equal(t, universityFive, universityFromChainFive)
	require.Equal(t, []codec.Address{}, studentsFromChainFive)

	// Check that action six worked
	require.Equal(t, nameSix, nameFromChainSix)
	require.Equal(t, yearSix, yearFromChainSix)
	require.Equal(t, universitySix, universityFromChainSix)
	require.Equal(t, []codec.Address{}, studentsFromChainSix)
}

func TestAddStudents(t *testing.T) {
	prep := prepare(t)

	parser, err := prep.instance.lcli.Parser(context.Background())
	require.NoError(t, err)

	submit, _, _, err := prep.instance.cli.GenerateTransaction(
		context.Background(),
		parser,
		[]chain.Action{
			&actions.AddProfessor{
				Name:       nameOne,
				Year:       yearOne,
				University: universityOne,
			},
			&actions.AddProfessor{
				Name:       nameTwo,
				Year:       yearTwo,
				University: universityTwo,
			},
			&actions.AddProfessor{
				Name:       nameThree,
				Year:       yearThree,
				University: universityThree,
			},
			&actions.AddProfessor{
				Name:       nameFour,
				Year:       yearFour,
				University: universityFour,
			},
		},
		factory,
	)

	require.NoError(t, err)
	require.NoError(t, submit(context.Background()))

	results := expectBlk(prep.instance)(false)
	require.Len(t, results, 1)
	require.True(t, results[0].Success)

	professorOneID := string(results[0].Outputs[0][0])

	submit, _, _, err = prep.instance.cli.GenerateTransaction(
		context.Background(),
		parser,
		[]chain.Action{
			&actions.AddStudent{
				ProfessorName: nameOne,
				StudentName:   nameTwo,
			},
			&actions.AddStudent{
				ProfessorName: nameOne,
				StudentName:   nameThree,
			},
		},
		factory,
	)

	require.NoError(t, err)
	require.NoError(t, submit(context.Background()))

	results = expectBlk(prep.instance)(false)
	require.Len(t, results, 1)
	require.True(t, results[0].Success)

	_, _, _, newStudents, _ := prep.instance.lcli.GetProfessorDetails(context.Background(), professorOneID)
	require.Equal(t, storage.GenerateProfessorID(nameTwo), newStudents[0])
	require.Equal(t, storage.GenerateProfessorID(nameThree), newStudents[1])
}

func TestThreeLayerTree(t *testing.T) {
	prep := prepare(t)

	parser, err := prep.instance.lcli.Parser(context.Background())
	require.NoError(t, err)

	submit, _, _, err := prep.instance.cli.GenerateTransaction(
		context.Background(),
		parser,
		[]chain.Action{
			&actions.AddProfessor{
				Name:       nameOne,
				Year:       yearOne,
				University: universityOne,
			},
			&actions.AddProfessor{
				Name:       nameTwo,
				Year:       yearTwo,
				University: universityTwo,
			},
			&actions.AddProfessor{
				Name:       nameThree,
				Year:       yearThree,
				University: universityThree,
			},
			&actions.AddProfessor{
				Name:       nameFour,
				Year:       yearFour,
				University: universityFour,
			},
			&actions.AddStudent{
				ProfessorName: nameOne,
				StudentName:   nameTwo,
			},
			&actions.AddStudent{
				ProfessorName: nameOne,
				StudentName:   nameThree,
			},
			&actions.AddStudent{
				ProfessorName: nameTwo,
				StudentName:   nameFour,
			},
		},
		factory,
	)

	require.NoError(t, err)
	require.NoError(t, submit(context.Background()))

	results := expectBlk(prep.instance)(false)
	require.Len(t, results, 1)
	require.True(t, results[0].Success)

	// professorOneID := string(results[0].Outputs[0][0])
	professorTwoID := string(results[0].Outputs[1][0])
	// professorThreeID := string(results[0].Outputs[2][0])
	// professorFourID := string(results[0].Outputs[3][0])

	_, _, _, newStudents, _ := prep.instance.lcli.GetProfessorDetails(context.Background(), professorTwoID)
	require.Equal(t, storage.GenerateProfessorID(nameFour), newStudents[0])
}

func TestLineageSearch(t *testing.T) {
	prep := prepare(t)

	parser, err := prep.instance.lcli.Parser(context.Background())
	require.NoError(t, err)

	submit, _, _, err := prep.instance.cli.GenerateTransaction(
		context.Background(),
		parser,
		[]chain.Action{
			&actions.AddProfessor{
				Name:       nameOne,
				Year:       yearOne,
				University: universityOne,
			},
			&actions.AddProfessor{
				Name:       nameTwo,
				Year:       yearTwo,
				University: universityTwo,
			},
			&actions.AddProfessor{
				Name:       nameThree,
				Year:       yearThree,
				University: universityThree,
			},
			&actions.AddProfessor{
				Name:       nameFour,
				Year:       yearFour,
				University: universityFour,
			},
			&actions.AddStudent{
				ProfessorName: nameOne,
				StudentName:   nameTwo,
			},
			&actions.AddStudent{
				ProfessorName: nameOne,
				StudentName:   nameThree,
			},
			&actions.AddStudent{
				ProfessorName: nameTwo,
				StudentName:   nameFour,
			},
		},
		factory,
	)

	require.NoError(t, err)
	require.NoError(t, submit(context.Background()))

	results := expectBlk(prep.instance)(false)
	require.Len(t, results, 1)
	require.True(t, results[0].Success)

	doesLineageExist, err := prep.instance.lcli.DoesLineageExist(context.Background(), nameOne, nameTwo)
	require.NoError(t, err)
	require.True(t, doesLineageExist)

	doesLineageExist, err = prep.instance.lcli.DoesLineageExist(context.Background(), nameOne, nameFour)
	require.NoError(t, err)
	require.True(t, doesLineageExist)

	doesLineageExist, err = prep.instance.lcli.DoesLineageExist(context.Background(), nameThree, nameFour)
	require.NoError(t, err)
	require.False(t, doesLineageExist)
}

func TestMultilayerTree(t *testing.T) {
	prep := prepare(t)

	parser, err := prep.instance.lcli.Parser(context.Background())
	require.NoError(t, err)

	submit, _, _, err := prep.instance.cli.GenerateTransaction(
		context.Background(),
		parser,
		[]chain.Action{
			&actions.AddProfessor{
				Name:       nameOne,
				Year:       yearOne,
				University: universityOne,
			},
			&actions.AddProfessor{
				Name:       nameTwo,
				Year:       yearTwo,
				University: universityTwo,
			},
			&actions.AddProfessor{
				Name:       nameThree,
				Year:       yearThree,
				University: universityThree,
			},
			&actions.AddProfessor{
				Name:       nameFour,
				Year:       yearFour,
				University: universityFour,
			},
			&actions.AddProfessor{
				Name:       nameFive,
				Year:       yearFive,
				University: universityFive,
			},
			&actions.AddProfessor{
				Name:       nameSix,
				Year:       yearSix,
				University: universitySix,
			},
			&actions.AddStudent{
				ProfessorName: nameOne,
				StudentName:   nameTwo,
			},
			&actions.AddStudent{
				ProfessorName: nameOne,
				StudentName:   nameThree,
			},
			&actions.AddStudent{
				ProfessorName: nameTwo,
				StudentName:   nameFour,
			},
			&actions.AddStudent{
				ProfessorName: nameTwo,
				StudentName:   nameFive,
			},
			&actions.AddStudent{
				ProfessorName: nameFour,
				StudentName:   nameSix,
			},
			&actions.AddStudent{
				ProfessorName: nameFive,
				StudentName:   nameSix,
			},
		},
		factory,
	)

	require.NoError(t, err)
	require.NoError(t, submit(context.Background()))

	results := expectBlk(prep.instance)(false)
	require.Len(t, results, 1)
	require.True(t, results[0].Success)

	doesLineageExist, err := prep.instance.lcli.DoesLineageExist(context.Background(), nameOne, nameSix)
	require.NoError(t, err)
	require.True(t, doesLineageExist)
}
