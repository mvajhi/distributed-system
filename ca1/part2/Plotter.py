import pandas as pd
import matplotlib.pyplot as plt

df = pd.read_csv('./results/results.csv')
for workload in ['cpu', 'mixed']:
    plt.figure()
    for g in [1,2,16]:
        sub = df[(df['Workload']==workload) & (df['GOMAXPROCS']==g)]
        plt.plot(sub['Goroutines'], sub['Throughput_ops_sec'], marker='o', label=f'GOMAXPROCS={g}')
    plt.xlabel('Number of Goroutines')
    plt.ylabel('Throughput (ops/sec)')
    plt.title(f'Throughput vs Goroutines - {workload} workload')
    plt.legend()
    plt.grid(True)
    plt.savefig(f'throughput_{workload}.png')
    plt.show()