app=captiveportal
distdir=$(HOME)/${app}

build:
	go mod tidy
	go build -o ${app}

rc:
	install -m 755 rc.${app} /etc/rc.d/${app}
	echo "${app} install in rc.d"
	echo "use rcctl enable|start ${app} to enable and start."

install:
	mkdir -p ${distdir}
	install -m 755 ${app} ${distdir}/
	cp -r static ${distdir}/
	cp -r templates ${distdir}/

dist:
	make build
	make install
	cd ${distdir}
	cd ..
	tar -czvf ${app}.tar.gz ${app}
	ls -l ${app}.tar.gz

clean:
	rm -rf ${distdir}
	rm -f ${app}
	rm -f /etc/rc.d/${app}
