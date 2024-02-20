/*
    image-manage is helper tool for managing vm templates on s3 storage
    Copyright (C) 2024  Adam Prycki

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/


package main

import "os"
import "log"
import "fmt"
import "time"
import "flag"
import "context"
import "io/ioutil"
import "encoding/json"
import "github.com/minio/minio-go/v7"
import "github.com/minio/minio-go/v7/pkg/credentials"

type Conf struct {
	Endpoint string
	AccessKey string
	SecretKey string
	HTTPS bool
	DefaultExpiryTime uint
	DefaultBucket string
	DefaultTimeoutMS uint
}

var config Conf

func write_example_config( path string ){
	if _, err := os.Stat( path ); err == nil {
		log.Printf(" path %s exists, cannot write example files\n", path)
		os.Exit(1)
	}
	
	example_config := Conf{
		Endpoint: "localhost:9000",
		AccessKey: "xxxxxxxxxxxxxxxxxxxx",
		SecretKey: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		HTTPS: false,
		DefaultExpiryTime: 48,
		DefaultBucket: "my-vm-images",
		DefaultTimeoutMS: 1000,
		}
	
	confser, err := json.MarshalIndent( example_config ,"","	")
	if err != nil {
		fmt.Println("Can't serislize", example_config )
		}
	ioutil.WriteFile( path, confser,0600)}

func load_config( path string ){
	file, err := os.Open( path )
	if err != nil {
		log.Println(err)
		os.Exit(10)}
	defer file.Close()
	raw, err := ioutil.ReadAll(file)
	if err != nil {
		log.Printf("ERR '%s' reading %s", err, path )
		os.Exit(10);}
	json.Unmarshal( raw, &config )
	}

var s3_client *minio.Client

func s3_setup_client() {
	var err error
	s3_client, err = minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4( config.AccessKey, config.SecretKey, "" ),
		Secure: config.HTTPS,
	})
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
}

var preamble string = `image-manager Copyright (C) 2024  Adam Prycki
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it
under certain conditions.

Usage:`

var TODO_preamble string = `	image-manager  Copyright (C) 2024  Adam Prycki
	This program comes with ABSOLUTELY NO WARRANTY; for details type 'show w'.
	This is free software, and you are welcome to redistribute it
	under certain conditions; type 'show c' for details.
	
	Usage:`


func s3_check_object_expiry( bucket string, object string, remove_expired bool ) bool {
	
	ctx, cancelCtx := context.WithTimeout( context.Background(), time.Duration( config.DefaultTimeoutMS ) * time.Millisecond )
	defer cancelCtx()
	
	objInfo, err := s3_client.StatObject( ctx, bucket, object, minio.StatObjectOptions{} ) 
	if err != nil {
		log.Printf("error %#v\n", err)
		os.Exit(1)
	}else{
		if debug {
			readable,_ := json.Marshal( objInfo )
			fmt.Println( string(readable) )
			fmt.Println( objInfo.LastModified )
		}
		if  time.Now().After( objInfo.LastModified.Add( time.Hour * time.Duration( config.DefaultExpiryTime ))) {
			if ! quiet {
				fmt.Printf("%s/%s expired\n", bucket, object)
			}
			if remove_expired {
				if ! quiet {
					fmt.Printf("Removing object %s/%s\n", bucket, object)
				}
				ctx, cancelCtx := context.WithTimeout( context.Background(), time.Duration( config.DefaultTimeoutMS ) * time.Millisecond )
				defer cancelCtx()
				err := s3_client.RemoveObject(ctx, bucket, object, minio.RemoveObjectOptions{} )
				if err != nil{
					log.Printf("Error while deleting object %s/%s : %s\n", bucket, object, err)
				}
			}
			//return( true )
			os.Exit(1)
		}else{
			if ! quiet {
				fmt.Printf("%s/%s not-expired\n", bucket, object)
			}
			return( false )
			os.Exit(0)
		}
	}
	return( false )
}

var config_path string
var b_write_example_config bool
var check_obj_expiry bool
var expiry_hours uint
var cmdline_bucket string
var cmdline_object string
var remove_expired bool
var debug bool
var quiet bool

func main() {
	
	flag.CommandLine.SetOutput( os.Stdout )
	flag.Usage = func() {
		w := flag.CommandLine.Output() // may be os.Stderr - but not necessarily
		//w := os.Stdout // may be os.Stderr - but not necessarily
		fmt.Fprintf(w, preamble)
		flag.PrintDefaults()
		//fmt.Fprintf(w, "...custom postamble ... \n")
		
	}
	
	flag.StringVar( &config_path, "config_path", "./config.json", "path to config file")
	flag.BoolVar( &b_write_example_config, "write_example_config", false, "write example config")
	flag.BoolVar( &debug, "debug", false, "show debug information")
	flag.BoolVar( &check_obj_expiry, "check_obj_expiry", false, "check if image is over -expiry threshold")
	flag.BoolVar( &remove_expired, "remove_expired", false, "remove object if image is over -expiry threshold")
	flag.UintVar( &expiry_hours, "expiry_hours", 48, "image expiry time in hours")
	flag.BoolVar( &quiet, "quiet", false, "run in quiet mode")
	flag.StringVar( &cmdline_bucket, "bucket", "my-bucket", "bucket name")
	flag.StringVar( &cmdline_object, "object", "my-image", "path to image object")
	flag.Parse()
	flagset := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { flagset[f.Name]=true } )
	if debug {
		quiet = false
		log.Printf("%+v\n", flagset)
	}
	
	// s3_check_object_expiry( "vm-images", "vm-template.raw" )
	if b_write_example_config && flagset["config_path"] {
		write_example_config( config_path )
		log.Printf( "writing example config to %s\n", config_path )
		os.Exit(0)
	}else if b_write_example_config && ! flagset["config_path"] { 
		log.Println( "Cannot write example config without -config_path\n" )
		os.Exit(2)
	}
	
	if flagset["config_path"] {
		load_config( config_path )
	}else{
		load_config( "./config.json" )
	}
	
	if debug {
		log.Printf("config: %+v\n", config)
	}
	// merge config and cmdline
	if flagset[ "cmdline_bucket" ] {
		config.DefaultBucket = cmdline_bucket
	}
	if flagset[ "expiry_hours" ] {
		config.DefaultExpiryTime = expiry_hours
	}
	
	if check_obj_expiry{
		if ! flagset["object"] {
			log.Println( "missing -object provide object name to check" )
			os.Exit(1)}
		if ( flagset["bucket"] || len(config.DefaultBucket) == 0 ) {
			log.Println( "missing bucket, provide it wth bucket or via config file" )
			os.Exit(1)}
			
		s3_setup_client()
		s3_check_object_expiry( config.DefaultBucket , cmdline_object, remove_expired )
	}
}
