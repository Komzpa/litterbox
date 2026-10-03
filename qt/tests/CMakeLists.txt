cmake_minimum_required(VERSION 3.22)
project(CardStoreTests LANGUAGES CXX)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_AUTOMOC ON)
find_package(Qt6 REQUIRED COMPONENTS Core Sql Qml Quick QuickControls2 Network Test)
find_package(KF6Kirigami CONFIG REQUIRED)
enable_testing()
add_executable(tst_cardstore tst_cardstore.cpp ../src/CardStore.cpp ../src/CardStore.h)
target_include_directories(tst_cardstore PRIVATE ../src)
target_link_libraries(tst_cardstore PRIVATE Qt6::Core Qt6::Sql Qt6::Qml Qt6::Quick Qt6::QuickControls2 Qt6::Network Qt6::Test KF6::Kirigami)
target_compile_definitions(tst_cardstore PRIVATE LB_SOURCE_ACTIONS_QML="${CMAKE_CURRENT_SOURCE_DIR}/../src/qml/CardActions.qml")
add_test(NAME cardstore COMMAND tst_cardstore)
set_tests_properties(cardstore PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")

# Opt-in measurement (not an offscreen CTest): on an owned systemd-run Xvfb,
# python3 mailresize_driver.py --runner <build>/tst_mailresize --display :N --output <proof-dir>
# reports 10-second real-mail resizes and inbox/open/close/archive/create heartbeat gaps.
# --imports <5fadb81>/qt/tests/qml-imports repeats the same oracle as a negative control.
if(NOT ANDROID)
    find_package(Qt6 REQUIRED COMPONENTS QuickTest WebEngineQuick)
    add_executable(tst_mailresize maildetail_runner.cpp ../src/maildocumentprofile.cpp ../src/maildocumentprofile.h
                   ../src/CardStore.cpp ../src/CardStore.h)
    target_include_directories(tst_mailresize PRIVATE ../src)
    target_compile_definitions(tst_mailresize PRIVATE LB_RESIZE_STORE
                              LB_MAIL_FIXTURES_DIR="${CMAKE_CURRENT_SOURCE_DIR}/fixtures")
    target_link_libraries(tst_mailresize PRIVATE Qt6::QuickTest Qt6::WebEngineQuick Qt6::Sql KF6::Kirigami)
endif()

# Real CardStore consumer proof; optionally load a copied desktop cache.
find_package(Qt6 REQUIRED COMPONENTS QuickTest)
add_executable(tst_bundleexpand bundle_runner.cpp ../src/CardStore.cpp ../src/CardStore.h)
target_include_directories(tst_bundleexpand PRIVATE ../src)
target_link_libraries(tst_bundleexpand PRIVATE Qt6::QuickTest Qt6::Sql KF6::Kirigami)
add_test(NAME bundleexpand COMMAND tst_bundleexpand
    -input ${CMAKE_CURRENT_SOURCE_DIR}/tst_bundleexpand.qml
    -import ${CMAKE_CURRENT_SOURCE_DIR}/qml-imports)
set_tests_properties(bundleexpand PROPERTIES ENVIRONMENT
    "QT_QPA_PLATFORM=offscreen;QT_QUICK_CONTROLS_STYLE=org.kde.desktop")
