package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"math"

	"github.com/piotrkowalczuk/ntypes"
	"github.com/piotrkowalczuk/pqt"
	"github.com/piotrkowalczuk/qtypes"
	"github.com/piotrkowalczuk/sklog"
)

//go:generate generator
//go:generate goimports -w schema.pqt.go

var (
	address string
	dbg     bool
)

func init() {
	flag.StringVar(&address, "addr", "postgres://localhost:5432/test?sslmode=disable", "postgres connection string")
	flag.BoolVar(&dbg, "dbg", true, "debug mode")
}

func main() {
	flag.Parse()
	log := sklog.NewHumaneLogger(os.Stdout, sklog.DefaultHTTPFormatter)
	db, err := sql.Open("postgres", address)
	if err != nil {
		sklog.Fatal(log, err)
	}

	_, err = db.Exec(SQL)
	if err != nil {
		sklog.Fatal(log, err)
	}

	repo := struct {
		news     newsRepositoryBase
		comment  commentRepositoryBase
		category categoryRepositoryBase
	}{
		news: newsRepositoryBase{
			db:      db,
			table:   tableNews,
			columns: tableNewsColumns,
			dbg:     true,
			log:     log,
		},
		comment: commentRepositoryBase{
			db:      db,
			table:   tableComment,
			columns: tableCommentColumns,
			dbg:     true,
			log:     log,
		},
		category: categoryRepositoryBase{
			db:      db,
			table:   tableCategory,
			columns: tableCategoryColumns,
			dbg:     true,
			log:     log,
		},
	}

	count, err := repo.news.count(&newsCriteria{})
	if err != nil {
		sklog.Fatal(log, err)
	}
	sklog.Debug(log, "number of news fetched", "count", count)

	news, err := repo.news.insert(&newsEntity{
		title:   "Lorem Ipsum",
		content: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Nullam a felis vel erat gravida luctus at id nisi. Cum sociis natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Vivamus a nibh massa. Cum sociis natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Fusce viverra quam id dolor facilisis ultrices. Donec blandit, justo sit amet consequat gravida, nisi velit efficitur neque, ac ullamcorper leo dui vitae lorem. Pellentesque vitae ligula id massa fringilla facilisis eu sit amet neque. Ut ac fringilla mi. Maecenas id fermentum massa. Duis at tristique felis, nec aliquet nisi. Suspendisse potenti. In sed dolor maximus, dapibus arcu vitae, vehicula ligula. Nunc imperdiet eu ipsum sed pretium. Nullam iaculis nunc id dictum auctor.",
		lead:    &ntypes.String{String: "Neque porro quisquam est qui dolorem ipsum quia dolor sit amet, consectetur, adipisci velit...", Valid: true},
	})
	if err != nil {
		switch pqt.ErrorConstraint(err) {
		case tableNewsConstraintTitleUnique:
			sklog.Fatal(log, errors.New("news with such title already exists"))
		default:
			sklog.Fatal(log, err)
		}
	}

	nb := 20
	for i := 0; i < nb; i++ {
		_, err = repo.comment.insert(&commentEntity{
			newsID:    news.id,
			newsTitle: news.title,
			content:   "Etiam eget nunc vel tellus placerat accumsan. Quisque dictum commodo orci, a eleifend nulla viverra malesuada. Etiam dui purus, dapibus a risus sed, porta scelerisque lorem. Sed vehicula mauris tellus, at dapibus risus facilisis vitae. Sed at lacus mollis, cursus sapien eu, egestas ligula. Cras blandit, arcu quis aliquam dictum, nibh purus pulvinar turpis, in dapibus est nibh et enim. Donec ex arcu, iaculis eget euismod id, lobortis nec enim. Quisque sed massa vel dui convallis ultrices. Nulla rutrum sed lacus vel ornare. Aliquam vulputate condimentum elit at pellentesque. Curabitur vitae sem tincidunt, volutpat urna ut, consequat turpis. Pellentesque varius justo libero, a volutpat lacus vulputate at. Integer tristique pharetra urna vel pharetra. In porttitor tincidunt eros, vel eleifend quam elementum a.",
		})
		if err != nil {
			sklog.Fatal(log, err)
		}
	}

	iter, err := repo.comment.findIter(&commentCriteria{
		newsID: qtypes.EqualInt64(news.id),
		sort: map[string]bool{
			"id": false,
			"non_existing_column": true,
		},
	})
	if err != nil {
		sklog.Fatal(log, err)
	}
	got := 0
	for iter.Next() {
		com, err := iter.Comment()
		if err != nil {
			sklog.Fatal(log, err)
		}
		sklog.Debug(log, "comment fetched", "comment_id", com.id)
		got++
	}
	if err = iter.Err(); err != nil {
		sklog.Fatal(log, err)
	}
	if nb != got {
		sklog.Fatal(log, fmt.Errorf("wrong number of comments, expected %d but got %d", nb, got))
	} else {
		sklog.Info(log, "proper number of comments")
	}

	category, err := repo.category.insert(&categoryEntity{
		name: "parent",
	})
	if err != nil {
		sklog.Fatal(log, err)
	}

	for i := 0; i < nb; i++ {
		_, err := repo.category.insert(&categoryEntity{
			parentID: &ntypes.Int64{Int64: category.id, Valid: true},
			name:     "child_category" + strconv.Itoa(i),
		})
		if err != nil {
			sklog.Fatal(log, err)
		}
	}

	count, err = repo.category.count(&categoryCriteria{
		parentID: qtypes.EqualInt64(category.id),
	})
	if err != nil {
		sklog.Fatal(log, err)
	}
	if count != int64(nb) {
		sklog.Fatal(log, fmt.Errorf("wrong number of categories, expected %d but got %d", nb, count))
	} else {
		sklog.Info(log, "proper number of categories")
	}

	_, err = repo.category.insert(&categoryEntity{
		parentID: &ntypes.Int64{Int64: int64(math.MaxInt64 - 1), Valid: true},
		name:     "does not work",
	})
	if err != nil {
		switch pqt.ErrorConstraint(err) {
		case tableCategoryConstraintParentIDForeignKey:
			sklog.Info(log, "category parent id constraint properly catched, category with such id does not exists")
		default:
			sklog.Fatal(log, fmt.Errorf("category constraint not catched properly, expected %s but got %s", tableCategoryConstraintParentIDForeignKey, pqt.ErrorConstraint(err)))
		}
	}
}
