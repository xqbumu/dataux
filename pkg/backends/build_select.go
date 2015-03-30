package backends

import (
	"fmt"
	"strings"

	u "github.com/araddon/gou"
	"github.com/araddon/qlbridge/datasource"
	"github.com/araddon/qlbridge/exec"
	"github.com/araddon/qlbridge/expr"
	"github.com/araddon/qlbridge/value"
	"github.com/dataux/dataux/pkg/models"
)

func (m *Builder) VisitSelect(stmt *expr.SqlSelect) (interface{}, error) {
	u.Debugf("VisitSelect %+v", stmt)

	if sysVar := stmt.SysVariable(); len(sysVar) > 0 {
		return m.VisitSysVariable(stmt)
	} else if len(stmt.From) == 0 && len(stmt.Columns) == 1 && strings.ToLower(stmt.Columns[0].As) == "database" {
		return m.VisitSelectDatabase(stmt)
	}

	tasks := make(exec.Tasks, 0)

	if len(stmt.From) == 1 {
		var from *expr.SqlSource
		if len(stmt.From) > 1 {
			return nil, fmt.Errorf("join not implemented")
		} else if len(stmt.From) == 1 {
			from = stmt.From[0]
		}

		// source is of type qlbridge.datasource.DataSource
		source, err := m.schema.DataSource.SourceTask(stmt)
		if err != nil {
			return nil, err
		}
		// Some data sources provide their own projections
		if projector, ok := source.(models.SourceProjection); ok {
			m.Projection, err = projector.Projection()
			if err != nil {
				u.Errorf("could not build projection %v", err)
				return nil, err
			}
		} else {
			panic("must implement projection")
		}
		if scanner, ok := source.(datasource.Scanner); !ok {
			return nil, fmt.Errorf("Must Implement Scanner")
		} else {
			sourceTask := exec.NewSource(from.Name, scanner)
			tasks.Add(sourceTask)
		}
	} else {
		// for now, only support 1 join
		if len(stmt.From) != 2 {
			return nil, fmt.Errorf("3 or more Table/Join not currently implemented")
		}
		// u.Debugf("we are going to do a join on two dbs: ")
		// for _, from := range stmt.From {
		// 	u.Infof("from:  %#v", from)
		// }

		stmt.From[0].Source = stmt.From[0].Rewrite(stmt)
		stmt.From[1].Source = stmt.From[1].Rewrite(stmt)
		u.Debugf("from1: %v", stmt.From[0].Source.String())
		u.Debugf("from2: %v", stmt.From[1].Source.String())

		in, err := exec.NewSourceJoin(stmt.From[0], stmt.From[1], m.svr.RtConf)
		if err != nil {
			return nil, err
		}
		tasks.Add(in)
	}

	return tasks, nil
}

func (m *Builder) VisitSelectDatabase(stmt *expr.SqlSelect) (interface{}, error) {
	u.Debugf("VisitSelectDatabase %+v", stmt)

	tasks := make(exec.Tasks, 0)
	val := "NULL"
	if m.schema != nil {
		val = m.schema.Db
	}
	static := datasource.NewStaticDataValue(val, "database")
	sourceTask := exec.NewSource("system", static)
	tasks.Add(sourceTask)
	m.Projection = StaticProjection("database", value.StringType)
	return tasks, nil
}