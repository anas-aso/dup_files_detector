use clap::{arg, command, ArgAction};
use std::{
    collections::{hash_map::DefaultHasher, HashMap},
    hash::{Hash, Hasher},
    io,
    process::exit,
    str::FromStr,
};
use walkdir::WalkDir;

fn main() {
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
        .map(|d| d.as_str())
        .collect::<Vec<_>>();

    let delete_dups = matches.get_flag("deleteDuplicates");
    delete_warning(delete_dups);

    println!(
        "Processing files in the following directory(ies): {:?}",
        dir_paths
    );

    let files_with_same_size = get_files_with_same_size(dir_paths, matches.get_flag("ignoreEmpty"));

    let result = get_identical_files(files_with_same_size);

    print_results(result);
}

fn get_files_with_same_size(dirs: Vec<&str>, ignore_empty: bool) -> HashMap<u64, Vec<String>> {
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

    return result;
}

fn get_identical_files(
    files_with_same_size: HashMap<u64, Vec<String>>,
) -> HashMap<u64, Vec<String>> {
    let mut result: HashMap<u64, Vec<String>> = HashMap::new();

    files_with_same_size.values().for_each(|files_group| {
        files_group.iter().for_each(|file_path| {
            let file_content = std::fs::read(file_path).unwrap();
            let digest = calculate_hash(&file_content);

            add_file_path(&mut result, digest, file_path.clone());
        });
    });

    return result;
}

fn calculate_hash<T: Hash>(t: &T) -> u64 {
    let mut s = DefaultHasher::new();
    t.hash(&mut s);
    s.finish()
}

fn add_file_path(result: &mut HashMap<u64, Vec<String>>, id: u64, value: String) {
    match result.get_mut(&id) {
        Some(files_group) => files_group.push(value),
        None => {
            let mut files_group: Vec<String> = Vec::new();
            files_group.push(value);
            result.insert(id, files_group);
        }
    }
}

fn print_results(result: HashMap<u64, Vec<String>>) {
    result.keys().for_each(|hash| {
        let files_paths = result.get(hash).unwrap();
        let dup_files_count = files_paths.len();
        if dup_files_count > 1 {
            eprintln!("{hash} ({dup_files_count})");
            files_paths.iter().for_each(|path| eprintln!("{path}"));
            eprintln!()
        }
    });
}

fn delete_warning(delete_dups: bool) {
    if delete_dups {
        println!("WARNING: deleting duplicated files is enabled. Do you want to continue ? (y/N) ");
        let mut response = String::new();
        io::stdin().read_line(&mut response).unwrap();
        let response = response.trim();
        if response != "y" {
            println!("Abort ...");
            exit(0);
        }
    }
}
