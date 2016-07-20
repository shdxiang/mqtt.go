/*
 * Copyright (c) 2013 IBM Corp.
 *
 * All rights reserved. This program and the accompanying materials
 * are made available under the terms of the Eclipse Public License v1.0
 * which accompanies this distribution, and is available at
 * http://www.eclipse.org/legal/epl-v10.html
 *
 * Contributors:
 *    Seth Hoenig
 *    Allan Stockdill-Mander
 *    Mike Robertson
 */

package mqtt

import (
	"log"
	"os"
)

var (
	ERROR    *log.Logger
	CRITICAL *log.Logger
	WARN     *log.Logger
	DEBUG    *log.Logger
)

func init() {
	nolog, err := os.Open("/dev/null")
	if err != nil {
		log.Fatal(err)
	}

	ERROR = log.New(nolog, "", 0)
	CRITICAL = log.New(nolog, "", 0)
	WARN = log.New(nolog, "", 0)
	DEBUG = log.New(nolog, "", 0)
}
