import os
from tqdm import tqdm
import platform
import datetime

linux_bin = "separa"
win_bin = "separa.exe"

def read_target(file: str) -> list[str]:
    res: list[str] = []
    with open(file, 'r') as f:
        for line in f.readlines():
            res.append(line.strip())
    return res


def scan_target():
    pass


if __name__ == '__main__':
    system = platform.system()
    folder_name = datetime.datetime.now().strftime("%Y%m%d%H%M")

    targets = read_target('target.txt')
    print("load " + str(len(targets)) + " CIDR to scan")
    for target in tqdm(targets):

        output = os.path.join('outputs', folder_name, target[:-3] + ".json")

        if system == 'Windows':
            os.system(win_bin + " -t " + target + " -o " + output)
        elif system == 'Linux':
            os.system(linux_bin + " -t " + target + " -o " + output)