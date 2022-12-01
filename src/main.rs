use std::{
    collections::{hash_map::DefaultHasher, HashMap},
    fs::{read, remove_file},
    hash::{Hash, Hasher},
    io::stdin,
    process::exit,
    str::FromStr,
};

use clap::{arg, command, ArgAction};
use walkdir::WalkDir;

struct CliArgs {
    dir_paths: Vec<String>,
    ignore_empty: bool,
    delete_dups: bool,
}

fn main() {
    let args = parse_args();

    deletion_warning(args.delete_dups);

    println!(
        "Processing files in the following directory(ies): {:?}",
        args.dir_paths
    );

    let files_with_same_size = get_files_with_same_size(args.dir_paths, args.ignore_empty);
    let result = get_identical_files(files_with_same_size);

    process_results(result, args.delete_dups);
}

// parse cli input arguments/flags and return them as CliArgs object
fn parse_args() -> CliArgs {
    let matches = command!()
        .arg(
            arg!(
            --directoryPath <PATH> "Path to the directory you want to check (repeatable)."
            )
            .default_value("./")
            .required(false)
            .action(ArgAction::Append),
        )
        .arg(arg!(
        --ignoreEmpty "Ignore empty files."
        ))
        .arg(arg!(
        --deleteDuplicates "Delete found duplicates (use with caution!)."
        ))
        .get_matches();

    let dir_paths = matches
        .get_many::<String>("directoryPath")
        .unwrap()
        .cloned()
        .collect::<Vec<String>>();

    let delete_dups = matches.get_flag("deleteDuplicates");
    let ignore_empty = matches.get_flag("ignoreEmpty");

    CliArgs {
        dir_paths,
        ignore_empty,
        delete_dups,
    }
}

// print a warning and ask for user confirmation if the deletion flag is set
fn deletion_warning(delete_dups: bool) {
    if delete_dups {
        println!("WARNING: deleting duplicated files is enabled. Do you want to continue ? (y/N) ");
        let mut response = String::new();
        stdin().read_line(&mut response).unwrap();
        let response = response.trim();
        if response != "y" {
            println!("Abort ...");
            exit(0);
        }
    }
}

// group files with the same size.
// this optimization is needed to avoid calculating checksum for
// files without duplicates which is expensive for large files.
fn get_files_with_same_size(dirs: Vec<String>, ignore_empty: bool) -> HashMap<u64, Vec<String>> {
    let mut result: HashMap<u64, Vec<String>> = HashMap::new();
    for dir in dirs {
        for fd in WalkDir::new(dir) {
            let fd = fd.unwrap();
            let fd_meta = fd.metadata().unwrap();

            if fd_meta.is_file() {
                let file_size = fd_meta.len();
                // skip empty files if desired
                if ignore_empty && file_size == 0 {
                    continue;
                }

                let file_path = String::from_str(fd.path().to_str().unwrap()).unwrap();
                add_file_path(&mut result, file_size, file_path);
            }
        }
    }

    result
}

// add an element to the map array value.
// create a new map entry if no corresponding key exists
fn add_file_path(result: &mut HashMap<u64, Vec<String>>, id: u64, value: String) {
    match result.get_mut(&id) {
        Some(files_group) => files_group.push(value),
        None => {
            let files_group: Vec<String> = vec![value];
            result.insert(id, files_group);
        }
    }
}

// group files with the same checksum.
// files with unique sizes will be skipped since they cannot have a duplicate.
fn get_identical_files(
    files_with_same_size: HashMap<u64, Vec<String>>,
) -> HashMap<u64, Vec<String>> {
    let mut result: HashMap<u64, Vec<String>> = HashMap::new();

    files_with_same_size.values().for_each(|files_group| {
        // ignore files with unique size.
        if files_group.len() > 1 {
            files_group.iter().for_each(|file_path| {
                let file_content = read(file_path).unwrap();
                let digest = calculate_hash(&file_content);

                add_file_path(&mut result, digest, file_path.clone());
            });
        }
    });

    result
}

// calculate the hash of an input object
fn calculate_hash<T: Hash>(t: &T) -> u64 {
    let mut s = DefaultHasher::new();
    t.hash(&mut s);
    s.finish()
}

// process the map of files with duplicates and print results.
// this function will also perform the deletion of duplicates if instructed.
fn process_results(result: HashMap<u64, Vec<String>>, delete_dups: bool) {
    result.keys().for_each(|hash| {
        let files_paths = result.get(hash).unwrap();
        let dup_files_count = files_paths.len();
        if dup_files_count > 1 {
            eprintln!("{hash} ({dup_files_count})");
            let mut keep = true;
            files_paths.iter().for_each(|path| {
                eprint!("{path}\t");
                if delete_dups && !keep {
                    eprint!("...\tDeleting duplicate.");
                    remove_file(path).expect("Failed to remove duplicate file!");
                }
                keep = false;
                eprintln!()
            });
            eprintln!()
        }
    });
}
