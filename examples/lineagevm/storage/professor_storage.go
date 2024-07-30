// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"container/list"
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	lconsts "github.com/ava-labs/hypersdk/examples/lineagevm/consts"
)

// [balancePrefix] + [address]
func ProfessorStateKey(addr codec.Address) []byte {
	k := make([]byte, 1+codec.AddressLen+consts.Uint16Len)
	k[0] = professorStatePrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], ProfessorStateChunks)
	return k
}

// Returns the associated professor ID, generated from a name
func GenerateProfessorID(name string) codec.Address {
	id := utils.ToID([]byte(name))
	return codec.CreateAddress(lconsts.PROFESSORID, id)
}

func AddProfessorComplex(
	ctx context.Context,
	mu state.Mutable,
	name string,
	year uint16,
	university string,
) (codec.Address, error) {
	professorID := GenerateProfessorID(name)
	professorStateKey := ProfessorStateKey(professorID)

	// Check that professor state does not already exist
	_, err := mu.GetValue(ctx, professorStateKey)
	if err == nil {
		return codec.EmptyAddress, errors.New("professor already exists")
	} else if !errors.Is(err, database.ErrNotFound) {
		return codec.EmptyAddress, err
	}

	// Write to DB
	// Name Length (uint16) | name | Year (uint16) | University Length (uint16)
	// | Address List (irrelevant for initialization)
	// Similar logic to TokenVM
	nameLength := len(name)
	universityLength := len(university)

	// TODO: handle chunk sizing rules
	// nameLength | name | yearLength | universityLength | university
	valueLength := consts.Uint16Len + nameLength + consts.Uint16Len + consts.Uint16Len + universityLength + consts.Uint16Len

	v := make([]byte, valueLength)
	// Inserting name
	binary.BigEndian.PutUint16(v, uint16(nameLength))
	copy(v[getNameIndex():], []byte(name))
	// Inserting year
	binary.BigEndian.PutUint16(v[getYearIndex(nameLength):], year)
	// Inserting university
	binary.BigEndian.PutUint16(v[getUniversityLengthIndex(nameLength):], uint16(universityLength))
	copy(v[getUniversityIndex(nameLength):], []byte(university))
	// Inserting number of students
	binary.BigEndian.PutUint16(v[getNumOfStudentsIndex(nameLength, universityLength):], 0)

	err = mu.Insert(ctx, professorStateKey, v)
	if err != nil {
		return codec.EmptyAddress, err
	}

	return professorID, nil
}

func GetProfessorFromStateComplex(
	ctx context.Context,
	f ReadState,
	professorID codec.Address,
) (bool, string, uint16, string, []codec.Address, error) {
	k := ProfessorStateKey(professorID)
	values, errs := f(ctx, [][]byte{k})
	return innerGetProfessorFromState(values[0], errs[0])
}

func innerGetProfessorFromState(
	v []byte,
	err error,
) (bool, string, uint16, string, []codec.Address, error) {
	if errors.Is(err, database.ErrNotFound) {
		return false, "", 0, "", []codec.Address{}, nil
	}
	if err != nil {
		return false, "", 0, "", []codec.Address{}, err
	}
	// Extracting name
	nameLen := binary.BigEndian.Uint16(v)
	name := string(v[getNameIndex():getYearIndex(int(nameLen))])
	// Extracting year
	year := binary.BigEndian.Uint16(v[getYearIndex(int(nameLen)):])
	// Extracting university
	universityLen := binary.BigEndian.Uint16(v[getUniversityLengthIndex(int(nameLen)):])
	university := string(v[getUniversityIndex(int(nameLen)):getNumOfStudentsIndex(int(nameLen), int(universityLen))])

	numOfStudents := binary.BigEndian.Uint16(v[getNumOfStudentsIndex(int(nameLen), int(universityLen)):])

	students := []codec.Address{}

	// Range over each student and add to students array
	for i := 0; i < int(numOfStudents); i++ {
		startIndex, endIndex := getStudentIndices(int(nameLen), int(universityLen), i)
		currStudent := v[startIndex:endIndex]

		students = append(students, codec.Address(currStudent))
	}

	success := true
	return success, name, year, university, students, nil
}

func AddStudentToState(
	ctx context.Context,
	mu state.Mutable,
	professorName string,
	studentName string,
) error {
	professorID := GenerateProfessorID(professorName)
	studentID := GenerateProfessorID(studentName)
	kp := ProfessorStateKey(professorID)
	ks := ProfessorStateKey(studentID)

	// Check that Professor exists
	_, err := mu.GetValue(ctx, kp)
	// TODO: handle case where database does not exist
	if err != nil {
		return err
	}

	// Check that Student exists
	_, err = mu.GetValue(ctx, ks)
	if err != nil {
		return err
	}

	return innerAddStudentToState(ctx, mu, professorID, studentID)
}

func innerAddStudentToState(
	ctx context.Context,
	mu state.Mutable,
	professorID codec.Address,
	studentID codec.Address,
) error {
	// First, read professor state
	v, _ := mu.GetValue(ctx, ProfessorStateKey(professorID))
	nv := make([]byte, len(v)+codec.AddressLen)
	copy(nv, v)

	// Get name, university lengths
	// Extracting name
	nameLen := binary.BigEndian.Uint16(v)
	universityLen := binary.BigEndian.Uint16(v[getUniversityLengthIndex(int(nameLen)):])

	numOfStudents := binary.BigEndian.Uint16(v[getNumOfStudentsIndex(int(nameLen), int(universityLen)):])
	// Get index for new student (i.e. we are appending)
	newStudentIndex, _ := getStudentIndices(int(nameLen), int(universityLen), int(numOfStudents))
	// Write student to professor state and add back to DB
	copy(nv[newStudentIndex:], studentID[:])
	// Update number of students
	numOfStudents += 1
	binary.BigEndian.PutUint16(nv[getNumOfStudentsIndex(int(nameLen), int(universityLen)):], numOfStudents)
	mu.Insert(ctx, ProfessorStateKey(professorID), nv)

	return nil
}

func getNameIndex() int {
	return consts.Uint16Len
}

func getYearIndex(nameLength int) int {
	return getNameIndex() + nameLength
}

func getUniversityLengthIndex(nameLength int) int {
	return getYearIndex(nameLength) + consts.Uint16Len
}

func getUniversityIndex(nameLength int) int {
	return getUniversityLengthIndex(nameLength) + consts.Uint16Len
}

func getNumOfStudentsIndex(nameLength int, universityLength int) int {
	return getUniversityIndex(nameLength) + universityLength
}

func getStartOfStudentListIndex(nameLength int, universityLength int) int {
	return getNumOfStudentsIndex(nameLength, universityLength) + consts.Uint16Len
}

func getStudentIndices(nameLength int, universityLength int, studentNum int) (startIndex int, endIndex int) {
	startIndex = getStartOfStudentListIndex(nameLength, universityLength) + (codec.AddressLen * studentNum)
	endIndex = startIndex + codec.AddressLen
	return
}

// Uses BFS
func DoesLineageExist(
	ctx context.Context,
	f ReadState,
	professorName string,
	studentName string,
) (existsPath bool, err error) {
	// First, check that Professor, Student exist in state
	professorID := GenerateProfessorID(professorName)
	studentID := GenerateProfessorID(studentName)
	professorStateKey := ProfessorStateKey(professorID)
	studentStateKey := ProfessorStateKey(studentID)

	_, errs := f(ctx, [][]byte{professorStateKey})
	if errs[0] != nil {
		return false, errs[0]
	}

	_, errs = f(ctx, [][]byte{studentStateKey})
	if errs[0] != nil {
		return false, errs[0]
	}

	// Conduct BFS starting with professor
	q := list.New()
	q.PushBack(professorID)
	for q.Len() != 0 {
		e := q.Front()
		if e.Value == studentID {
			return true, nil
		}
		// Remove e from list, add students
		q.Remove(e)
		_, _, _, _, studentList, err := GetProfessorFromStateComplex(ctx, f, e.Value.(codec.Address))
		if err != nil {
			return false, err
		}
		for _, s := range studentList {
			q.PushBack(s)
		}
	}

	existsPath = false
	err = nil

	return
}
